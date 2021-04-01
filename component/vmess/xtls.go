package vmess

import (
	xtls "github.com/xtls/go"
	"net"
)

type XTLSConfig struct {
	Host           string
	SkipCertVerify bool
	SessionCache   xtls.ClientSessionCache
	NextProtos     []string
}

func StreamXTLSConn(conn net.Conn, cfg *XTLSConfig) (net.Conn, error) {
	xtlsConfig := &xtls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: cfg.SkipCertVerify,
		ClientSessionCache: cfg.SessionCache,
		NextProtos:         cfg.NextProtos,
	}

	xtlsConn := xtls.Client(conn, xtlsConfig)
	xtlsConn.RPRX = true
	xtlsConn.SHOW = true
	xtlsConn.MARK = "XTLS"
	xtlsConn.DirectMode = true

	err := xtlsConn.Handshake()
	return xtlsConn, err
}
