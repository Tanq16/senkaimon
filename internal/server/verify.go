package server

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/policy"
	"github.com/Tanq16/senkaimon/internal/store"
)

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	host := policy.NormalizeHost(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		http.Error(w, "X-Forwarded-Host is missing", http.StatusBadRequest)
		return
	}
	requestPath := r.Header.Get("X-Forwarded-Uri")

	bearer, hasBearer := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	var principal store.Principal
	switch {
	case hasBearer:
		p, err := s.store.ResolveToken(strings.TrimSpace(bearer))
		if err != nil {
			s.record(audit.Event{Event: audit.TokenDenied, IP: clientIP(r), Host: host, Detail: err.Error()})
			w.Header().Set("WWW-Authenticate", `Bearer realm="senkaimon"`)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		principal = p
	default:
		p, ok := s.sessionPrincipal(r)
		if !ok {
			s.challenge(w, r, host, requestPath)
			return
		}
		principal = p
	}

	verdict := policy.Decide(s.store.Policies(), principal.Subject, host, requestPath)
	if !verdict.Allowed {
		s.record(audit.Event{
			Event:   audit.AccessDenied,
			Subject: principal.Subject,
			IP:      clientIP(r),
			Host:    host,
			Detail:  verdict.Reason,
		})
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	w.Header().Set(headerUser, principal.Owner)
	w.Header().Set(headerKind, string(principal.Kind))
	if principal.Kind == store.KindToken {
		w.Header().Set(headerTokenID, principal.Subject)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) challenge(w http.ResponseWriter, r *http.Request, host, requestPath string) {
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	rd := proto + "://" + host + requestPath
	target := *s.idp
	target.Path = "/login"
	target.RawQuery = url.Values{"rd": {rd}}.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (s *Server) redirectAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return false
	}
	host := policy.NormalizeHost(u.Host)
	// A policy rule of "allow *" would otherwise allowlist every host on the internet.
	if !s.withinCookieDomain(host) {
		return false
	}
	if host == policy.NormalizeHost(s.idp.Host) {
		return true
	}
	for _, glob := range policy.Hosts(s.store.Policies()) {
		if ok, err := path.Match(glob, host); err == nil && ok {
			return true
		}
	}
	return false
}

func (s *Server) withinCookieDomain(host string) bool {
	domain := strings.TrimPrefix(strings.ToLower(s.cfg.Identity.CookieDomain), ".")
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func (s *Server) resolveRedirect(raw string) string {
	if raw != "" && s.redirectAllowed(raw) {
		return raw
	}
	return s.cfg.Identity.IDPURL
}
