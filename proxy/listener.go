package proxy

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"sync"

	"github.com/Dreamacro/clash/component/resolver"
	"github.com/Dreamacro/clash/config"
	"github.com/Dreamacro/clash/dns"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/proxy/http"
	"github.com/Dreamacro/clash/proxy/mixed"
	"github.com/Dreamacro/clash/proxy/redir"
	"github.com/Dreamacro/clash/proxy/socks"
	"github.com/Dreamacro/clash/proxy/tun"
)

var (
	allowLan    = false
	bindAddress = "*"

	socksListener     *socks.SockListener
	socksUDPListener  *socks.SockUDPListener
	httpListener      *http.HTTPListener
	redirListener     *redir.RedirListener
	redirUDPListener  *redir.RedirUDPListener
	tproxyListener    *redir.TProxyListener
	tproxyUDPListener *redir.RedirUDPListener
	mixedListener     *mixed.MixedListener
	mixedUDPLister    *socks.SockUDPListener
	tunAdapter        tun.TunAdapter

	// lock for recreate function
	socksMux  sync.Mutex
	httpMux   sync.Mutex
	redirMux  sync.Mutex
	tproxyMux sync.Mutex
	mixedMux  sync.Mutex
	tunMux    sync.Mutex
)

type Ports struct {
	Port       int `json:"port"`
	SocksPort  int `json:"socks-port"`
	RedirPort  int `json:"redir-port"`
	TProxyPort int `json:"tproxy-port"`
	MixedPort  int `json:"mixed-port"`
}

func AllowLan() bool {
	return allowLan
}

func BindAddress() string {
	return bindAddress
}

func SetAllowLan(al bool) {
	allowLan = al
}

func Tun() config.Tun {
	if tunAdapter == nil {
		return config.Tun{}
	}
	return config.Tun{
		Enable:         true,
		DeviceURL:      tunAdapter.DeviceURL(),
		DNSListen:      tunAdapter.DNSListen(),
		MacOSAutoRoute: true,
	}
}

func SetBindAddress(host string) {
	bindAddress = host
}

func ReCreateHTTP(port int) error {
	httpMux.Lock()
	defer httpMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	if httpListener != nil {
		if httpListener.Address() == addr {
			return nil
		}
		httpListener.Close()
		httpListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	httpListener, err = http.NewHTTPProxy(addr)
	if err != nil {
		return err
	}

	return nil
}

func ReCreateSocks(port int) error {
	socksMux.Lock()
	defer socksMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	shouldTCPIgnore := false
	shouldUDPIgnore := false

	if socksListener != nil {
		if socksListener.Address() != addr {
			socksListener.Close()
			socksListener = nil
		} else {
			shouldTCPIgnore = true
		}
	}

	if socksUDPListener != nil {
		if socksUDPListener.Address() != addr {
			socksUDPListener.Close()
			socksUDPListener = nil
		} else {
			shouldUDPIgnore = true
		}
	}

	if shouldTCPIgnore && shouldUDPIgnore {
		return nil
	}

	if portIsZero(addr) {
		return nil
	}

	tcpListener, err := socks.NewSocksProxy(addr)
	if err != nil {
		return err
	}

	udpListener, err := socks.NewSocksUDPProxy(addr)
	if err != nil {
		tcpListener.Close()
		return err
	}

	socksListener = tcpListener
	socksUDPListener = udpListener

	return nil
}

func ReCreateRedir(port int) error {
	redirMux.Lock()
	defer redirMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	if redirListener != nil {
		if redirListener.Address() == addr {
			return nil
		}
		redirListener.Close()
		redirListener = nil
	}

	if redirUDPListener != nil {
		if redirUDPListener.Address() == addr {
			return nil
		}
		redirUDPListener.Close()
		redirUDPListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	redirListener, err = redir.NewRedirProxy(addr)
	if err != nil {
		return err
	}

	redirUDPListener, err = redir.NewRedirUDPProxy(addr)
	if err != nil {
		log.Warnln("Failed to start Redir UDP Listener: %s", err)
	}

	return nil
}

func ReCreateTProxy(port int) error {
	tproxyMux.Lock()
	defer tproxyMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	if tproxyListener != nil {
		if tproxyListener.Address() == addr {
			return nil
		}
		tproxyListener.Close()
		tproxyListener = nil
	}

	if tproxyUDPListener != nil {
		if tproxyUDPListener.Address() == addr {
			return nil
		}
		tproxyUDPListener.Close()
		tproxyUDPListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	tproxyListener, err = redir.NewTProxy(addr)
	if err != nil {
		return err
	}

	tproxyUDPListener, err = redir.NewRedirUDPProxy(addr)
	if err != nil {
		log.Warnln("Failed to start TProxy UDP Listener: %s", err)
	}

	return nil
}

func ReCreateMixed(port int) error {
	mixedMux.Lock()
	defer mixedMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	shouldTCPIgnore := false
	shouldUDPIgnore := false

	if mixedListener != nil {
		if mixedListener.Address() != addr {
			mixedListener.Close()
			mixedListener = nil
		} else {
			shouldTCPIgnore = true
		}
	}
	if mixedUDPLister != nil {
		if mixedUDPLister.Address() != addr {
			mixedUDPLister.Close()
			mixedUDPLister = nil
		} else {
			shouldUDPIgnore = true
		}
	}

	if shouldTCPIgnore && shouldUDPIgnore {
		return nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	mixedListener, err = mixed.NewMixedProxy(addr)
	if err != nil {
		return err
	}

	mixedUDPLister, err = socks.NewSocksUDPProxy(addr)
	if err != nil {
		mixedListener.Close()
		return err
	}

	return nil
}

func ReCreateTun(conf config.Tun) error {
	tunMux.Lock()
	defer tunMux.Unlock()

	enable := conf.Enable
	url := conf.DeviceURL

	if tunAdapter != nil {
		if enable && (url == "" || url == tunAdapter.DeviceURL()) {
			// Though we don't need to recreate tun device, we should update tun DNSServer
			return tunAdapter.ReCreateDNSServer(resolver.DefaultResolver.(*dns.Resolver), resolver.DefaultHostMapper.(*dns.ResolverEnhancer), conf.DNSListen)
		}
		tunAdapter.Close()
		tunAdapter = nil
		if conf.MacOSAutoRoute {
			removeMacOSAutoRoute()
		}
	}
	if !enable {
		return nil
	}
	var err error
	tunAdapter, err = tun.NewTunProxy(url)
	if err != nil {
		return err
	}
	if resolver.DefaultResolver != nil {
		return tunAdapter.ReCreateDNSServer(resolver.DefaultResolver.(*dns.Resolver), resolver.DefaultHostMapper.(*dns.ResolverEnhancer), conf.DNSListen)
	}
	if conf.MacOSAutoRoute {
		setMacOSAutoRoute()
	}
	return nil
}

func setMacOSAutoRoute() {
	if runtime.GOOS != "darwin" {
		addSystemRoute("1")
		addSystemRoute("2/7")
		addSystemRoute("4/6")
		addSystemRoute("8/5")
		addSystemRoute("16/4")
		addSystemRoute("32/3")
		addSystemRoute("64/2")
		addSystemRoute("128.0/1")
		addSystemRoute("198.18.0/16")
	}
}

func removeMacOSAutoRoute() {
	if runtime.GOOS != "darwin" {
		delSystemRoute("1")
		delSystemRoute("2/7")
		delSystemRoute("4/6")
		delSystemRoute("8/5")
		delSystemRoute("16/4")
		delSystemRoute("32/3")
		delSystemRoute("64/2")
		delSystemRoute("128.0/1")
		delSystemRoute("198.18.0/16")
	}
}

func addSystemRoute(net string) {
	cmd := exec.Command("route", "add", "-net", net, "198.18.0.1")
	if err := cmd.Run(); err != nil {
		log.Errorln("[Add system route]Failed to add system route: %s", cmd.String())
	}
}

func delSystemRoute(net string) {
	cmd := exec.Command("route", "delete", "-net", net, "198.18.0.1")
	if err := cmd.Run(); err != nil {
		log.Errorln("[Delete system route]Failed to delete system route: %s", cmd.String())
	}
}

// GetPorts return the ports of proxy servers
func GetPorts() *Ports {
	ports := &Ports{}

	if httpListener != nil {
		_, portStr, _ := net.SplitHostPort(httpListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.Port = port
	}

	if socksListener != nil {
		_, portStr, _ := net.SplitHostPort(socksListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.SocksPort = port
	}

	if redirListener != nil {
		_, portStr, _ := net.SplitHostPort(redirListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.RedirPort = port
	}

	if tproxyListener != nil {
		_, portStr, _ := net.SplitHostPort(tproxyListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.TProxyPort = port
	}

	if mixedListener != nil {
		_, portStr, _ := net.SplitHostPort(mixedListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.MixedPort = port
	}

	return ports
}

func portIsZero(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return true
	}
	return false
}

func genAddr(host string, port int, allowLan bool) string {
	if allowLan {
		if host == "*" {
			return fmt.Sprintf(":%d", port)
		}
		return fmt.Sprintf("%s:%d", host, port)
	}

	return fmt.Sprintf("127.0.0.1:%d", port)
}
