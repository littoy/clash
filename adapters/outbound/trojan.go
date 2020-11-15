package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Dreamacro/clash/component/dialer"
	"github.com/Dreamacro/clash/component/trojan"
	C "github.com/Dreamacro/clash/constant"
	xtls "github.com/xtls/go"
)

type Trojan struct {
	*Base
	instance *trojan.Trojan
}

type TrojanOption struct {
	Name           string   `proxy:"name"`
	Server         string   `proxy:"server"`
	Port           int      `proxy:"port"`
	Password       string   `proxy:"password"`
	Flow           string   `proxy:"flow,omitempty"`
	ALPN           []string `proxy:"alpn,omitempty"`
	SNI            string   `proxy:"sni,omitempty"`
	SkipCertVerify bool     `proxy:"skip-cert-verify,omitempty"`
	UDP            bool     `proxy:"udp,omitempty"`
}

func (t *Trojan) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	c, err := t.instance.StreamConn(c)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}

	var tc trojan.Command
	if xtlsConn, ok := c.(*xtls.Conn); ok {
		xtlsConn.RPRX = true
		if t.instance.GetFlow() == trojan.XRD || t.instance.GetFlow() == trojan.XRD+"-udp443" {
			xtlsConn.DirectMode = true
			tc = trojan.CommandXRD
		} else {
			tc = trojan.CommandXRO
		}
	} else {
		tc = trojan.CommandTCP
	}
	err = t.instance.WriteHeader(c, tc, serializesSocksAddr(metadata))
	return c, err
}

func (t *Trojan) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}
	tcpKeepAlive(c)
	c, err = t.StreamConn(c, metadata)
	if err != nil {
		return nil, err
	}

	return NewConn(c, t), err
}

func (t *Trojan) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	if (t.instance.GetFlow() == trojan.XRD || t.instance.GetFlow() == trojan.XRO) && metadata.DstPort == "443" {
		return nil, fmt.Errorf("%s stopped UDP/443", t.instance.GetFlow())
	}
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	c, err := dialer.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}
	tcpKeepAlive(c)
	c, err = t.instance.StreamConn(c)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", t.addr, err)
	}

	err = t.instance.WriteHeader(c, trojan.CommandUDP, serializesSocksAddr(metadata))
	if err != nil {
		return nil, err
	}

	pc := t.instance.PacketConn(c)
	return newPacketConn(pc, t), err
}

func (t *Trojan) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": t.Type().String(),
	})
}

func NewTrojan(option TrojanOption) (*Trojan, error) {
	addr := net.JoinHostPort(option.Server, strconv.Itoa(option.Port))

	tOption := &trojan.Option{
		Password:            option.Password,
		Flow:                option.Flow,
		ALPN:                option.ALPN,
		ServerName:          option.Server,
		SkipCertVerify:      option.SkipCertVerify,
		ClientSessionCache:  getClientSessionCache(),
		ClientXSessionCache: getClientXSessionCache(),
	}

	if option.SNI != "" {
		tOption.ServerName = option.SNI
	}

	return &Trojan{
		Base: &Base{
			name: option.Name,
			addr: addr,
			tp:   C.Trojan,
			udp:  option.UDP,
		},
		instance: trojan.New(tOption),
	}, nil
}
