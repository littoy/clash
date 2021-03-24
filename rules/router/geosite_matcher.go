
package router

import (
	"errors"
	"fmt"
	"runtime"
	
	"github.com/Dreamacro/clash/common/strmatcher"
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

func NewDomainMatcher(domains []*Domain, country string) (*DomainMatcher, error) {
	
	if DomainMatcherCache[country] == nil {
		g := new(strmatcher.MatcherGroup)
		for _, d := range domains {
			m, err := domainToMatcher(d)
			if err != nil {
				return nil, err
			}
			g.Add(m)
		}

		matcherList := &DomainMatcher{
			matchers: g,
		}
		
		defer runtime.GC()
		DomainMatcherCache[country] = matcherList
		return matcherList, nil
	}
	
	return DomainMatcherCache[country], nil;
}

func (m *DomainMatcher) ApplyDomain(domain string) bool {
	return len(m.matchers.Match(domain)) > 0
}
