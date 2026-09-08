package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/Tanq16/senkaimon/internal/store"
)

type contextKey struct{}

var principalKey contextKey

func principalOf(r *http.Request) store.Principal {
	p, _ := r.Context().Value(principalKey).(store.Principal)
	return p
}

func (s *Server) sessionPrincipal(r *http.Request) (store.Principal, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return store.Principal{}, false
	}
	p, err := s.store.ResolveSession(cookie.Value)
	if err != nil {
		return store.Principal{}, false
	}
	return p, true
}

func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.sessionPrincipal(r)
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	}
}

func (s *Server) withAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.withSession(func(w http.ResponseWriter, r *http.Request) {
		p := principalOf(r)
		if p.Kind != store.KindUser || !p.Admin {
			writeError(w, http.StatusForbidden, "admin privileges required")
			return
		}
		next(w, r)
	})
}

func (s *Server) withOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			writeError(w, http.StatusForbidden, "missing Origin header")
			return
		}
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, s.idp.Host) || u.Scheme != s.idp.Scheme {
			writeError(w, http.StatusForbidden, "origin is not permitted")
			return
		}
		next(w, r)
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Domain:   s.cfg.Identity.CookieDomain,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.Session.AbsoluteTTL.Seconds()),
	})
}

func (s *Server) setPendingCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     pendingCookie,
		Value:    value,
		Domain:   s.cfg.Identity.CookieDomain,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.Session.PendingTTL.Seconds()),
	})
}

func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Domain:   s.cfg.Identity.CookieDomain,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
