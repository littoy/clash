
package router

import (
	"errors"
	"fmt"
	"runtime"
	
	"github.com/Dreamacro/clash/common/strmatcher"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

var (
	DomainMatcherCache = make(map[string]*DomainMatcher)

	matcherTypeMap = map[Domain_Type]strmatcher.Type{
		Domain_Plain:  strmatcher.Substr,
		Domain_Regex:  strmatcher.Regex,
		Domain_Domain: strmatcher.Domain,
		Domain_Full:   strmatcher.Full,
	}
)

func domainToMatcher(domain *Domain) (strmatcher.Matcher, error) {
	matcherType, f := matcherTypeMap[domain.Type]
	if !f {
		return nil, fmt.Errorf("unsupported domain type %d", domain.Type)
	}

	matcher, err := matcherType.New(domain.Value)
	if err != nil {
		return nil, errors.New("failed to create domain matcher")
	}

	return matcher, nil
}

type DomainMatcher struct {
	matchers strmatcher.IndexMatcher
}

// Initial or update GeoSite rules
func UpdateGeoSiteRule(newRules []C.Rule) {
	
	defer runtime.GC()
	for _, rule := range newRules {
		if rule.RuleType() == C.GEOSITE {
			
			country := rule.Payload()
			domains, err := loadGeositeWithAttr("geosite.dat", country)
			if err != nil {
				log.Errorln("Failed to load geosite: %s, base error: %s", country, err.Error())
				continue
			}

			g := new(strmatcher.MatcherGroup)
			for _, d := range domains {
				m, err := domainToMatcher(d)
				if err != nil {
					log.Errorln("Failed to create domain matcher with domain: %s, base error: %s", d, err.Error())
					continue
				}
				g.Add(m)
			}
			
			log.Infoln("Start initial geosite matcher %s", country)

			DomainMatcherCache[country] = &DomainMatcher{
				matchers: g,
			}
		}
	}
}

func NewDomainMatcher(country string) (*DomainMatcher, error) {
	
	if DomainMatcherCache[country] == nil {
		
		return nil, fmt.Errorf("[GeoSite] Miss domain matcher cache for country: %s", country)
	}
	
	return DomainMatcherCache[country], nil;
}

func (m *DomainMatcher) ApplyDomain(domain string) bool {
	return len(m.matchers.Match(domain)) > 0
}
