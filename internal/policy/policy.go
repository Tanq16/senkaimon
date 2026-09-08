package policy

import (
	"net"
	"path"
	"slices"
	"strings"
)

const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

type Rule struct {
	Effect string `json:"effect"`
	Host   string `json:"host"`
}

type Policy struct {
	Name     string   `json:"name"`
	Subjects []string `json:"subjects"`
	Rules    []Rule   `json:"rules"`
}

type Verdict struct {
	Allowed bool
	Reason  string
}

func NormalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func matches(pattern, host string) bool {
	ok, err := path.Match(pattern, host)
	return err == nil && ok
}

func Decide(policies []Policy, subject, host, requestPath string) Verdict {
	var rules []Rule
	for _, p := range policies {
		if slices.Contains(p.Subjects, subject) {
			rules = append(rules, p.Rules...)
		}
	}
	if len(rules) == 0 {
		return Verdict{Reason: "no policy"}
	}

	var allows []Rule
	for _, r := range rules {
		if r.Effect == EffectDeny && matches(r.Host, host) {
			return Verdict{Reason: "explicit deny"}
		}
		if r.Effect == EffectAllow {
			allows = append(allows, r)
		}
	}
	if len(allows) == 0 {
		return Verdict{Allowed: true, Reason: "deny-only policy"}
	}
	for _, r := range allows {
		if matches(r.Host, host) {
			return Verdict{Allowed: true, Reason: "allowed"}
		}
	}
	return Verdict{Reason: "not permitted"}
}

func Hosts(policies []Policy) []string {
	var hosts []string
	for _, p := range policies {
		for _, r := range p.Rules {
			if !slices.Contains(hosts, r.Host) {
				hosts = append(hosts, r.Host)
			}
		}
	}
	return hosts
}
