package shadowsocks

import (
	"net"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/tunnel"
	"github.com/Dreamacro/go-shadowsocks2/core"
)

type ShadowSockUDPListener struct {
	net.PacketConn
	address string
	closed  bool
}

func NewShadowSocksUDPProxy(addr string, cipher core.Cipher) (*ShadowSockUDPListener, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	pc := cipher.PacketConn(l)
	sl := &ShadowSockUDPListener{pc, addr, false}

	go func() {
		for {
			buf := pool.Get(pool.RelayBufferSize)
			n, rAddr, err := pc.ReadFrom(buf)
			if err != nil {
				_ = pool.Put(buf)
				if sl.closed {
					break
				}
				continue
			}
			handleShadowSocksUDP(rAddr, pc, buf[:n])
		}
	}()

	return sl, nil
}

func (l *ShadowSockUDPListener) Close() {
	l.closed = true
	_ = l.PacketConn.Close()
}

func (l *ShadowSockUDPListener) Address() string {
	return l.address
}

func handleShadowSocksUDP(rAddr net.Addr, pc net.PacketConn, buf []byte) {
	tAddr := socks5.SplitAddr(buf)
	payload := buf[len(tAddr):]

	packet := &packet{
		pc:      pc,
		rAddr:   rAddr,
		payload: payload,
		bufRef:  buf,
	}
	tunnel.AddPacket(adapters.NewPacket(tAddr, packet, C.SHADOWSOCKS))
}
