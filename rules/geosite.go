package rules

import (
	"github.com/Dreamacro/clash/component/mmdb"
	C "github.com/Dreamacro/clash/constant"
)

type GEOSITE struct {
	country     string
	adapter     string
	noResolveIP bool
}

func (g *GEOSITE) RuleType() C.RuleType {
	return C.GEOSITE
}

func (g *GEOSITE) Match(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if ip == nil {
		return false
	}
	record, _ := mmdb.Instance().Country(ip)
	return record.Country.IsoCode == g.country
}

func (g *GEOSITE) Adapter() string {
	return g.adapter
}

func (g *GEOSITE) Payload() string {
	return g.country
}

func (g *GEOSITE) ShouldResolveIP() bool {
	return !g.noResolveIP
}

func NewGEOSITE(country string, adapter string, noResolveIP bool) *GEOSITE {
	geosite := &GEOSITE{
		country:     country,
		adapter:     adapter,
		noResolveIP: noResolveIP,
	}

	return geosite
}
