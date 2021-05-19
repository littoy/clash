// +build windows

package dev

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	_ "unsafe"

	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/proxy/tun/dev/winipcfg"
	"github.com/Dreamacro/clash/proxy/tun/dev/wintun"
	"golang.org/x/sys/windows"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	rateMeasurementGranularity = uint64((time.Second / 2) / time.Nanosecond)
	spinloopRateThreshold      = 1000000000 / 8                                  // 1000mbps
	spinloopDuration           = uint64(time.Millisecond / 80 / time.Nanosecond) // ~1gbit/s

	messageTransportHeaderSize = 0 // size of data preceding content in transport message
)

type rateJuggler struct {
	current       uint64
	nextByteCount uint64
	nextStartTime int64
	changing      int32
}

type tunWindows struct {
	wt        *wintun.Adapter
	handle    windows.Handle
	closed    bool
	closing   sync.RWMutex
	forcedMTU int
	rate      rateJuggler
	session   wintun.Session
	readWait  windows.Handle
	stopOnce  sync.Once

	url       string
	name      string
	linkCache *channel.Endpoint
	wg        sync.WaitGroup // wait for goroutines to stop

	writeHandle *channel.NotificationHandle
}

var WintunPool, _ = wintun.MakePool("Clash")
var WintunStaticRequestedGUID *windows.GUID

//go:linkname procyield runtime.procyield
func procyield(cycles uint32)

//go:linkname nanotime runtime.nanotime
func nanotime() int64

// OpenTunDevice return a TunDevice according a URL
func OpenTunDevice(deviceURL url.URL) (TunDevice, error) {

	requestedGUID, err := windows.GUIDFromString("{330EAEF8-7578-5DF2-D97B-8DADC0EA85CB}")
	if err == nil {
		WintunStaticRequestedGUID = &requestedGUID
		log.Debugln("Generate GUID: %s", WintunStaticRequestedGUID.String())
	} else {
		log.Warnln("Error parese GUID from string: %v", err)
	}

	interfaceName := "Clash"
	mtu := 1420

	tun, err := CreateTUN(interfaceName, mtu)
	if err != nil {
		return nil, err
	}

	return tun, nil
}

//
// CreateTUN creates a Wintun interface with the given name. Should a Wintun
// interface with the same name exist, it is reused.
//
func CreateTUN(ifname string, mtu int) (TunDevice, error) {
	return CreateTUNWithRequestedGUID(ifname, WintunStaticRequestedGUID, mtu)
}

//
// CreateTUNWithRequestedGUID creates a Wintun interface with the given name and
// a requested GUID. Should a Wintun interface with the same name exist, it is reused.
//
func CreateTUNWithRequestedGUID(ifname string, requestedGUID *windows.GUID, mtu int) (TunDevice, error) {
	var err error
	var wt *wintun.Adapter

	// Does an interface with this name already exist?
	wt, err = WintunPool.OpenAdapter(ifname)
	if err == nil {
		// If so, we delete it, in case it has weird residual configuration.
		_, err = wt.Delete(false)
		if err != nil {
			return nil, fmt.Errorf("Error deleting already existing interface: %w", err)
		}
	}
	wt, rebootRequired, err := WintunPool.CreateAdapter(ifname, requestedGUID)
	if err != nil {
		return nil, fmt.Errorf("Error creating interface: %w", err)
	}
	if rebootRequired {
		log.Infoln("Windows indicated a reboot is required.")
	}

	forcedMTU := 1420
	if mtu > 0 {
		forcedMTU = mtu
	}

	tun := &tunWindows{
		wt:        wt,
		handle:    windows.InvalidHandle,
		forcedMTU: forcedMTU,
	}

	// config tun ip
	err = tun.configureInterface()
	if err != nil {
		tun.wt.Delete(false)
		return nil, fmt.Errorf("Error configure interface: %w", err)
	}

	realInterfaceName, err2 := wt.Name()
	if err2 == nil {
		ifname = realInterfaceName
		tun.name = realInterfaceName
	}

	tun.session, err = wt.StartSession(0x4000000) // Ring capacity, 64 MiB
	if err != nil {
		tun.wt.Delete(false)
		return nil, fmt.Errorf("Error starting session: %w", err)
	}
	tun.readWait = tun.session.ReadWaitEvent()
	return tun, nil
}

func (tun *tunWindows) getName() (string, error) {
	tun.closing.RLock()
	defer tun.closing.RUnlock()
	if tun.closed {
		return "", os.ErrClosed
	}
	return tun.wt.Name()
}

func (tun *tunWindows) Close() {
	tun.stopOnce.Do(func() {
		//tun.closing.Lock()
		//defer tun.closing.Unlock()
		tun.closed = true
		tun.session.End()
		if tun.wt != nil {
			forceCloseSessions := false
			rebootRequired, err := tun.wt.Delete(forceCloseSessions)
			if rebootRequired {
				log.Infoln("Delete Wintun failure, Windows indicated a reboot is required.")
			} else {
				log.Infoln("Delete Wintun success.")
			}
			if err != nil {
				log.Errorln("Close Wintun Sessions failure: %v", err)
			}
		}
	})
}

func (tun *tunWindows) MTU() (int, error) {
	return tun.forcedMTU, nil
}

// TODO: This is a temporary hack. We really need to be monitoring the interface in real time and adapting to MTU changes.
func (tun *tunWindows) ForceMTU(mtu int) {
	tun.forcedMTU = mtu
}

// Note: Read() and Write() assume the caller comes only from a single thread; there's no locking.

func (tun *tunWindows) Read(buff []byte, offset int) (int, error) {
	tun.closing.RLock()
	defer tun.closing.RUnlock()
retry:
	if tun.closed {
		return 0, os.ErrClosed
	}
	start := nanotime()
	shouldSpin := atomic.LoadUint64(&tun.rate.current) >= spinloopRateThreshold && uint64(start-atomic.LoadInt64(&tun.rate.nextStartTime)) <= rateMeasurementGranularity*2
	for {
		if tun.closed {
			return 0, os.ErrClosed
		}
		packet, err := tun.session.ReceivePacket()
		switch err {
		case nil:
			packetSize := len(packet)
			copy(buff[offset:], packet)
			tun.session.ReleaseReceivePacket(packet)
			tun.rate.update(uint64(packetSize))
			return packetSize, nil
		case windows.ERROR_NO_MORE_ITEMS:
			if !shouldSpin || uint64(nanotime()-start) >= spinloopDuration {
				windows.WaitForSingleObject(tun.readWait, windows.INFINITE)
				goto retry
			}
			procyield(1)
			continue
		case windows.ERROR_HANDLE_EOF:
			return 0, os.ErrClosed
		case windows.ERROR_INVALID_DATA:
			return 0, errors.New("Send ring corrupt")
		}
		return 0, fmt.Errorf("Read failed: %w", err)
	}
}

func (tun *tunWindows) Flush() error {
	return nil
}

func (tun *tunWindows) Write(buff []byte, offset int) (int, error) {
	tun.closing.RLock()
	defer tun.closing.RUnlock()
	if tun.closed {
		return 0, os.ErrClosed
	}

	packetSize := len(buff) - offset
	tun.rate.update(uint64(packetSize))

	packet, err := tun.session.AllocateSendPacket(packetSize)
	if err == nil {
		copy(packet, buff[offset:])
		tun.session.SendPacket(packet)
		return packetSize, nil
	}
	switch err {
	case windows.ERROR_HANDLE_EOF:
		return 0, os.ErrClosed
	case windows.ERROR_BUFFER_OVERFLOW:
		return 0, nil // Dropping when ring is full.
	}
	return 0, fmt.Errorf("Write failed: %w", err)
}

// LUID returns Windows interface instance ID.
func (tun *tunWindows) LUID() uint64 {
	tun.closing.RLock()
	defer tun.closing.RUnlock()
	if tun.closed {
		return 0
	}
	return tun.wt.LUID()
}

// RunningVersion returns the running version of the Wintun driver.
func (tun *tunWindows) RunningVersion() (version uint32, err error) {
	return wintun.RunningVersion()
}

func (rate *rateJuggler) update(packetLen uint64) {
	now := nanotime()
	total := atomic.AddUint64(&rate.nextByteCount, packetLen)
	period := uint64(now - atomic.LoadInt64(&rate.nextStartTime))
	if period >= rateMeasurementGranularity {
		if !atomic.CompareAndSwapInt32(&rate.changing, 0, 1) {
			return
		}
		atomic.StoreInt64(&rate.nextStartTime, now)
		atomic.StoreUint64(&rate.current, total*uint64(time.Second/time.Nanosecond)/period)
		atomic.StoreUint64(&rate.nextByteCount, 0)
		atomic.StoreInt32(&rate.changing, 0)
	}
}

func (tun *tunWindows) Name() string {
	return tun.name
}

func (t *tunWindows) URL() string {
	return fmt.Sprintf("dev://%s", t.Name())
}

// Wait wait goroutines to exit
func (t *tunWindows) Wait() {
	t.wg.Wait()
}

func (t *tunWindows) AsLinkEndpoint() (result stack.LinkEndpoint, err error) {
	if t.linkCache != nil {
		return t.linkCache, nil
	}

	mtu, err := t.MTU()

	if err != nil {
		return nil, errors.New("unable to get device mtu")
	}

	linkEP := channel.New(512, uint32(mtu), "")

	// start Read loop. read ip packet from tun and write it to ipstack
	t.wg.Add(1)
	go func() {
		for !t.closed {
			packet := make([]byte, mtu)
			n, err := t.Read(packet, messageTransportHeaderSize)
			if err != nil && !t.closed {
				log.Errorln("can not read from tun: %v", err)
			}
			var p tcpip.NetworkProtocolNumber
			switch header.IPVersion(packet) {
			case header.IPv4Version:
				p = header.IPv4ProtocolNumber
			case header.IPv6Version:
				p = header.IPv6ProtocolNumber
			}
			if linkEP.IsAttached() {
				linkEP.InjectInbound(p, stack.NewPacketBuffer(stack.PacketBufferOptions{
					Data: buffer.View(packet[:n]).ToVectorisedView(),
				}))
			} else {
				log.Debugln("received packet from tun when %s is not attached to any dispatcher.", t.Name())
			}
		}
		t.wg.Done()
		t.wt.Delete(false)
		log.Debugln("%v stop read loop", t.Name())
	}()

	// start write notification
	t.writeHandle = linkEP.AddNotify(t)
	t.linkCache = linkEP
	return t.linkCache, nil
}

// WriteNotify implements channel.Notification.WriteNotify.
func (t *tunWindows) WriteNotify() {
	packet, ok := t.linkCache.Read()
	if ok {
		var vv buffer.VectorisedView
		// Append upper headers.
		vv.AppendView(packet.Pkt.NetworkHeader().View())
		vv.AppendView(packet.Pkt.TransportHeader().View())
		// Append data payload.
		vv.Append(packet.Pkt.Data().ExtractVV())

		_, err := t.Write(vv.ToView(), messageTransportHeaderSize)
		if err != nil && !t.closed {
			log.Errorln("can not write to tun: %v", err)
		}
	}
}

func (tun *tunWindows) configureInterface() error {
	luid := winipcfg.LUID(tun.LUID())

	mtu, err := tun.MTU()

	if err != nil {
		return errors.New("unable to get device mtu")
	}

	family := winipcfg.AddressFamily(windows.AF_INET)
	familyV6 := winipcfg.AddressFamily(windows.AF_INET6)

	tunAddress := winipcfg.ParseIPCidr("198.18.0.1/16")

	addresses := []net.IPNet{tunAddress.IPNet()}

	allowedIPs := []*winipcfg.IPCidr{
		winipcfg.ParseIPCidr("1.0.0.0/8"),
		winipcfg.ParseIPCidr("2.0.0.0/7"),
		winipcfg.ParseIPCidr("4.0.0.0/6"),
		winipcfg.ParseIPCidr("8.0.0.0/5"),
		winipcfg.ParseIPCidr("16.0.0.0/4"),
		winipcfg.ParseIPCidr("32.0.0.0/3"),
		winipcfg.ParseIPCidr("64.0.0.0/2"),
		winipcfg.ParseIPCidr("128.0.0.0/1"),
		//winipcfg.ParseIPCidr("198.18.0.0/16"),
		//winipcfg.ParseIPCidr("198.18.0.1/32"),
		//winipcfg.ParseIPCidr("198.18.255.255/32"),
		winipcfg.ParseIPCidr("224.0.0.0/4"),
		winipcfg.ParseIPCidr("255.255.255.255/32"),
	}

	estimatedRouteCount := len(allowedIPs)
	routes := make([]winipcfg.RouteData, 0, estimatedRouteCount)
	var haveV4Address, haveV6Address bool = true, false

	foundDefault4 := false
	foundDefault6 := false
	for _, allowedip := range allowedIPs {
		allowedip.MaskSelf()
		if (allowedip.Bits() == 32 && !haveV4Address) || (allowedip.Bits() == 128 && !haveV6Address) {
			continue
		}
		route := winipcfg.RouteData{
			Destination: allowedip.IPNet(),
			Metric:      0,
		}
		if allowedip.Bits() == 32 {
			if allowedip.Cidr == 0 {
				foundDefault4 = true
			}
			route.NextHop = net.IPv4zero
		} else if allowedip.Bits() == 128 {
			if allowedip.Cidr == 0 {
				foundDefault6 = true
			}
			route.NextHop = net.IPv6zero
		}
		routes = append(routes, route)
	}

	err = luid.FlushIPAddresses(familyV6)
	if err != nil {
		return err
	}
	err = luid.FlushDNS(family)
	if err != nil {
		return err
	}
	err = luid.FlushDNS(familyV6)
	if err != nil {
		return err
	}
	err = luid.FlushRoutes(familyV6)
	if err != nil {
		return err
	}

	err = luid.SetIPAddressesForFamily(family, addresses)

	if err == windows.ERROR_OBJECT_ALREADY_EXISTS {
		cleanupAddressesOnDisconnectedInterfaces(family, addresses)
		err = luid.SetIPAddressesForFamily(family, addresses)
	}
	if err != nil {
		return err
	}

	deduplicatedRoutes := make([]*winipcfg.RouteData, 0, len(routes))
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Metric != routes[j].Metric {
			return routes[i].Metric < routes[j].Metric
		}
		if c := bytes.Compare(routes[i].NextHop, routes[j].NextHop); c != 0 {
			return c < 0
		}
		if c := bytes.Compare(routes[i].Destination.IP, routes[j].Destination.IP); c != 0 {
			return c < 0
		}
		if c := bytes.Compare(routes[i].Destination.Mask, routes[j].Destination.Mask); c != 0 {
			return c < 0
		}
		return false
	})
	for i := 0; i < len(routes); i++ {
		if i > 0 && routes[i].Metric == routes[i-1].Metric &&
			bytes.Equal(routes[i].NextHop, routes[i-1].NextHop) &&
			bytes.Equal(routes[i].Destination.IP, routes[i-1].Destination.IP) &&
			bytes.Equal(routes[i].Destination.Mask, routes[i-1].Destination.Mask) {
			continue
		}
		deduplicatedRoutes = append(deduplicatedRoutes, &routes[i])
	}

	err = luid.SetRoutesForFamily(family, deduplicatedRoutes)
	if err != nil {
		return err
	}

	ipif, err := luid.IPInterface(family)
	if err != nil {
		return err
	}

	ipif.NLMTU = uint32(mtu)

	if family == windows.AF_INET {
		if foundDefault4 {
			ipif.UseAutomaticMetric = false
			ipif.Metric = 0
		}
	} else if family == windows.AF_INET6 {
		if foundDefault6 {
			ipif.UseAutomaticMetric = false
			ipif.Metric = 0
		}
		ipif.DadTransmits = 0
		ipif.RouterDiscoveryBehavior = winipcfg.RouterDiscoveryDisabled
	}
	err = ipif.Set()
	if err != nil {
		return err
	}

	ipif6, err := luid.IPInterface(familyV6)
	if err != nil {
		return err
	}
	err = ipif6.Set()
	if err != nil {
		return err
	}

	return luid.SetDNS(family, []net.IP{net.ParseIP("198.18.0.2")}, nil)
}

func cleanupAddressesOnDisconnectedInterfaces(family winipcfg.AddressFamily, addresses []net.IPNet) {
	if len(addresses) == 0 {
		return
	}
	includedInAddresses := func(a net.IPNet) bool {
		// TODO: this makes the whole algorithm O(n^2). But we can't stick net.IPNet in a Go hashmap. Bummer!
		for _, addr := range addresses {
			ip := addr.IP
			if ip4 := ip.To4(); ip4 != nil {
				ip = ip4
			}
			mA, _ := addr.Mask.Size()
			mB, _ := a.Mask.Size()
			if bytes.Equal(ip, a.IP) && mA == mB {
				return true
			}
		}
		return false
	}
	interfaces, err := winipcfg.GetAdaptersAddresses(family, winipcfg.GAAFlagDefault)
	if err != nil {
		return
	}
	for _, iface := range interfaces {
		if iface.OperStatus == winipcfg.IfOperStatusUp {
			continue
		}
		for address := iface.FirstUnicastAddress; address != nil; address = address.Next {
			ip := address.Address.IP()
			ipnet := net.IPNet{IP: ip, Mask: net.CIDRMask(int(address.OnLinkPrefixLength), 8*len(ip))}
			if includedInAddresses(ipnet) {
				log.Infoln("[Wintun] Cleaning up stale address %s from interface ‘%s’", ipnet.String(), iface.FriendlyName())
				iface.LUID.DeleteIPAddress(ipnet)
			}
		}
	}
}

// GetAutoDetectInterface get ethernet interface
func GetAutoDetectInterface() (string, error) {
	ifname, err := getAutoDetectInterfaceByFamily(winipcfg.AddressFamily(windows.AF_INET))
	if err == nil {
		return ifname, err
	}

	return getAutoDetectInterfaceByFamily(winipcfg.AddressFamily(windows.AF_INET6))
}

func getAutoDetectInterfaceByFamily(family winipcfg.AddressFamily) (string, error) {
	interfaces, err := winipcfg.GetAdaptersAddresses(family, winipcfg.GAAFlagIncludeGateways)
	if err != nil {
		return "", fmt.Errorf("find ethernet interface failure. %w", err)
	}
	for _, iface := range interfaces {
		if iface.OperStatus != winipcfg.IfOperStatusUp {
			continue
		}

		ifname := iface.FriendlyName()
		if ifname == "Clash" {
			continue
		}

		for gatewayAddress := iface.FirstGatewayAddress; gatewayAddress != nil; gatewayAddress = gatewayAddress.Next {
			nextHop := gatewayAddress.Address.IP()

			var ipnet net.IPNet
			if family == windows.AF_INET {
				ipnet = net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}
			} else {
				ipnet = net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
			}

			if _, err = iface.LUID.Route(ipnet, nextHop); err == nil {
				return ifname, nil
			}
		}
	}

	return "", errors.New("ethernet interface not found")
}
