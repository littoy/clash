package rules

import (
	//"errors"
	"time"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

type GEOSITE struct {
	country     string
	adapter     string
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
	
	start := time.Now()
	
	domains, err := loadGeositeWithAttr("geosite.dat", country)
	if err != nil {
		log.Errorln("failed to load geosite: %s, base error: %s", country, err.Error())
		return false
	}
	
	matcher, err := NewDomainMatcher(domains)
	
	if err != nil {
		log.Errorln("failed to create DomainMatcher: %s", err.Error())
		return false
	}
	
	r := matcher.ApplyDomain(domain)
	
	elapsed := time.Since(start)
	
	log.Infoln("列表类型%d，域名%s匹配耗时: %s", domains[0].Type, domain, elapsed)
	
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
		country:     country,
		adapter:     adapter,
	}

	return geosite
}
