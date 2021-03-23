package rules

import (
	//"errors"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

type GEOSITE struct {
	country     string
	adapter     string
	noResolveIP bool
	domainType  string
}

func (g *GEOSITE) RuleType() C.RuleType {
	return C.GEOSITE
}

func (g *GEOSITE) Match(metadata *C.Metadata) bool {
	if metadata.AddrType != C.AtypDomainName {
		return false
	}
	
	domain := metadata.Host
	country := g.country
	
	domains, err := loadGeositeWithAttr("geosite.dat", country)
	if err != nil {
		//log.Infoln("HTTP proxy listening at: %s", addr)
		log.Errorln("failed to load geosite: %s, base error: %s", country, err.Error())
		return false
	}
	
	matcher, err := NewDomainMatcher(domains)
	
	if err != nil {
		log.Errorln("failed to create DomainMatcher: %s", err.Error())
		return false
	}
	
	return matcher.ApplyDomain(domain)
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

func NewGEOSITE(country string, adapter string, domainType string) *GEOSITE {
	geosite := &GEOSITE{
		country:     country,
		adapter:     adapter,
		noResolveIP: true,
		domainType:  domainType,
	}

	return geosite
}
