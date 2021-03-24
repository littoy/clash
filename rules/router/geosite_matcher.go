
package router

import (
	"errors"
	"fmt"
	"runtime"
	
	"github.com/Dreamacro/clash/common/strmatcher"
	//"github.com/Dreamacro/clash/log"
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

func NewDomainMatcher(country string) (*DomainMatcher, error) {
	
	if DomainMatcherCache[country] == nil {
		domains, err := loadGeositeWithAttr("geosite.dat", country)
		if err != nil {
			return nil, fmt.Errorf("Failed to load geosite: %s, base error: %s", country, err.Error())
		}
		
		g := new(strmatcher.MatcherGroup)
		for _, d := range domains {
			m, err := domainToMatcher(d)
			if err != nil {
				return nil, err
			}
			g.Add(m)
		}

		defer runtime.GC()
		DomainMatcherCache[country] = &DomainMatcher{
			matchers: g,
		}
		return DomainMatcherCache[country], nil
	}
	
	//log.Debugln("Using GeoSite matcher cache for country: %s", country)
	
	return DomainMatcherCache[country], nil;
}

func (m *DomainMatcher) ApplyDomain(domain string) bool {
	return len(m.matchers.Match(domain)) > 0
}
