package shadowsocks

import (
	"net"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/transport/socks5"
	"github.com/Dreamacro/clash/tunnel"
	"github.com/Dreamacro/go-shadowsocks2/core"
)

type ShadowSockListener struct {
	net.Listener
	address string
	closed  bool
}

func NewShadowSocksProxy(addr string, cipher core.Cipher) (*ShadowSockListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	sl := &ShadowSockListener{l, addr, false}

	go func() {
		log.Infoln("ShadowSocks proxy listening at: %s", addr)
		for {
			c, err := l.Accept()
			if err != nil {
				if sl.closed {
					break
				}
				continue
			}
			sc := cipher.StreamConn(c)
			go HandleShadowSocks(sc)
		}
	}()

	return sl, nil
}

func (l *ShadowSockListener) Close() {
	l.closed = true
	_ = l.Listener.Close()
}

func (l *ShadowSockListener) Address() string {
	return l.address
}

func HandleShadowSocks(c net.Conn) {
	tAddr, err := socks5.ReadAddr(c, make([]byte, socks5.MaxAddrLen))
	if err != nil {
		return
	}

	tunnel.Add(adapters.NewSocket(tAddr, c, C.SHADOWSOCKS))
}
