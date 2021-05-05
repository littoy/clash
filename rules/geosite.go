package rules

import (
	//"errors"
	//"time"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/rules/geosite"
)

type GEOSITE struct {
	country string
	adapter string
}

func (g *GEOSITE) RuleType() C.RuleType {
	return C.GEOSITE
}

func (g *GEOSITE) Match(metadata *C.Metadata) bool {
	if metadata.AddrType != C.AtypDomainName {
		return false
	}

	//start := time.Now()

	domain := metadata.Host

	matcher, err := geosite.NewDomainMatcher(g.country)

	if err != nil {
		log.Errorln("Failed to get geosite matcher for country: %s, base error: %s", g.country, err.Error())
		return false
	}

	r := matcher.ApplyDomain(domain)
	/**
	if r {
		elapsed := time.Since(start)
		log.Infoln("Match geosite domain \"%s\" spend time: %s", domain, elapsed)
	} **/

	return r
}

func (g *GEOSITE) Adapter() string {
	return g.adapter
}

func (g *GEOSITE) Payload() string {
	return g.country
}

func (g *GEOSITE) ShouldResolveIP() bool {
	return false
}

func NewGEOSITE(country string, adapter string) *GEOSITE {
	geosite := &GEOSITE{
		country: country,
		adapter: adapter,
	}

	return geosite
}
