package server

import (
	"context"
	"embed"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/ratelimit"
	"github.com/Tanq16/senkaimon/internal/store"
)

//go:embed static
var staticFiles embed.FS

const (
	sessionCookie = "senkaimon_session"
	pendingCookie = "senkaimon_pending"

	headerUser    = "Senkaimon-User"
	headerKind    = "Senkaimon-Kind"
	headerTokenID = "Senkaimon-Token-Id"
)

type Server struct {
	cfg     *store.Config
	store   *store.Store
	audit   *audit.Log
	limiter *ratelimit.Limiter
	idp     *url.URL
	mux     *http.ServeMux
	srv     *http.Server
}

func New(cfg *store.Config, st *store.Store, log *audit.Log) *Server {
	return &Server{
		cfg:     cfg,
		store:   st,
		audit:   log,
		limiter: ratelimit.New(cfg.RateLimit.MaxFailures, cfg.RateLimit.Window, cfg.RateLimit.Lockout),
		mux:     http.NewServeMux(),
	}
}

func (s *Server) Setup() error {
	idp, err := store.ParseIDPURL(s.cfg.Identity.IDPURL)
	if err != nil {
		return err
	}
	s.idp = idp

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /verify", s.handleVerify)

	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /api/login", s.withOrigin(s.handleLogin))
	s.mux.HandleFunc("GET /api/login/enrol", s.handleEnrolStart)
	s.mux.HandleFunc("POST /api/login/enrol", s.withOrigin(s.handleEnrolFinish))
	s.mux.HandleFunc("POST /api/login/totp", s.withOrigin(s.handleTOTPChallenge))
	s.mux.HandleFunc("POST /api/logout", s.withOrigin(s.handleLogout))

	s.mux.HandleFunc("GET /{$}", s.handleIndex)

	s.mux.HandleFunc("GET /api/account", s.withSession(s.handleAccount))
	s.mux.HandleFunc("POST /api/account/password", s.withOrigin(s.withSession(s.handleAccountPassword)))
	s.mux.HandleFunc("GET /api/account/sessions", s.withSession(s.handleAccountSessions))
	s.mux.HandleFunc("DELETE /api/account/sessions/{hash}", s.withOrigin(s.withSession(s.handleAccountSessionRevoke)))

	s.mux.HandleFunc("GET /api/admin/users", s.withAdmin(s.handleUsersList))
	s.mux.HandleFunc("POST /api/admin/users", s.withOrigin(s.withAdmin(s.handleUserCreate)))
	s.mux.HandleFunc("PATCH /api/admin/users/{username}", s.withOrigin(s.withAdmin(s.handleUserUpdate)))
	s.mux.HandleFunc("DELETE /api/admin/users/{username}", s.withOrigin(s.withAdmin(s.handleUserDelete)))
	s.mux.HandleFunc("POST /api/admin/users/{username}/reset-totp", s.withOrigin(s.withAdmin(s.handleUserResetTOTP)))

	s.mux.HandleFunc("GET /api/admin/tokens", s.withAdmin(s.handleTokensList))
	s.mux.HandleFunc("POST /api/admin/tokens", s.withOrigin(s.withAdmin(s.handleTokenMint)))
	s.mux.HandleFunc("DELETE /api/admin/tokens/{id}", s.withOrigin(s.withAdmin(s.handleTokenRevoke)))

	s.mux.HandleFunc("GET /api/admin/policies", s.withAdmin(s.handlePoliciesList))
	s.mux.HandleFunc("PUT /api/admin/policies/{name}", s.withOrigin(s.withAdmin(s.handlePolicyPut)))
	s.mux.HandleFunc("DELETE /api/admin/policies/{name}", s.withOrigin(s.withAdmin(s.handlePolicyDelete)))

	s.mux.HandleFunc("GET /api/admin/sessions", s.withAdmin(s.handleSessionsList))
	s.mux.HandleFunc("DELETE /api/admin/sessions/{hash}", s.withOrigin(s.withAdmin(s.handleSessionRevoke)))

	s.mux.HandleFunc("GET /api/admin/audit", s.withAdmin(s.handleAudit))

	s.srv = &http.Server{
		Addr:              s.cfg.Server.Listen,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return nil
}

func (s *Server) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.Session.FlushInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.store.Sweep()
				s.limiter.Sweep()
				if err := s.store.Flush(); err != nil {
					log.Error().Err(err).Msg("failed to flush state")
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("failed to shut down cleanly")
		}
	}()

	log.Info().Str("addr", s.cfg.Server.Listen).Str("idp", s.cfg.Identity.IDPURL).Msg("starting")
	err := s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if flushErr := s.store.Flush(); flushErr != nil {
		log.Error().Err(flushErr).Msg("failed to flush state on shutdown")
	}
	return err
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) servePage(w http.ResponseWriter, name string) {
	data, err := staticFiles.ReadFile("static/" + name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionPrincipal(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	s.servePage(w, "index.html")
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.servePage(w, "login.html")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.MarshalWrite(w, v); err != nil {
		log.Error().Err(err).Msg("failed to write response")
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	return json.UnmarshalRead(http.MaxBytesReader(nil, r.Body, 1<<20), v)
}

func (s *Server) record(e audit.Event) {
	if err := s.audit.Write(e); err != nil {
		log.Error().Err(err).Str("event", e.Event).Msg("failed to write audit event")
	}
}

func clientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return ""
	}
	first, _, _ := strings.Cut(forwarded, ",")
	return strings.TrimSpace(first)
}
