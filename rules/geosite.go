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
	network C.NetWork
}

func (gs *GEOSITE) RuleType() C.RuleType {
	return C.GEOSITE
}

func (gs *GEOSITE) Match(metadata *C.Metadata) bool {
	if metadata.AddrType != C.AtypDomainName {
		return false
	}

	//start := time.Now()

	domain := metadata.Host

	matcher, err := geosite.NewDomainMatcher(gs.country)

	if err != nil {
		log.Errorln("Failed to get geosite matcher for country: %s, base error: %s", gs.country, err.Error())
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

func (gs *GEOSITE) Adapter() string {
	return gs.adapter
}

func (gs *GEOSITE) Payload() string {
	return gs.country
}

func (gs *GEOSITE) ShouldResolveIP() bool {
	return false
}

func (gs *GEOSITE) NetWork() C.NetWork {
	return gs.network
}

func NewGEOSITE(country string, adapter string, network C.NetWork) *GEOSITE {
	geosite := &GEOSITE{
		country: country,
		adapter: adapter,
		network: network,
	}

	return geosite
}
