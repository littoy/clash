package shadowsocks

import (
	"bytes"
	"net"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
)

type packet struct {
	pc      net.PacketConn
	rAddr   net.Addr
	payload []byte
	bufRef  []byte
}

func (c *packet) Data() []byte {
	return c.payload
}

func (c *packet) WriteBack(b []byte, addr net.Addr) (n int, err error) {
	sAddr := socks5.ParseAddr(addr.String())
	packet := bytes.Join([][]byte{sAddr, b}, []byte{})

	return c.pc.WriteTo(packet, c.rAddr)
}

func (c *packet) LocalAddr() net.Addr {
	return c.rAddr
}

func (c *packet) Drop() {
	_ = pool.Put(c.bufRef)
}
