package server

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/store"
	"github.com/Tanq16/senkaimon/internal/totp"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Redirect string `json:"rd"`
}

type codeRequest struct {
	Code     string `json:"code"`
	Redirect string `json:"rd"`
}

func loginKeys(username, ip string) []string {
	keys := []string{"user:" + username}
	if ip != "" {
		keys = append(keys, "ip:"+ip)
	}
	return keys
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	ip := clientIP(r)
	if ip == "" {
		log.Warn().Msg("X-Forwarded-For is absent on /api/login, per-address rate limiting is inactive")
	}

	keys := loginKeys(req.Username, ip)
	if s.limiter.Locked(keys...) {
		s.record(audit.Event{Event: audit.LoginLocked, Subject: req.Username, IP: ip})
		writeError(w, http.StatusTooManyRequests, "too many failed attempts, try again later")
		return
	}

	user, ok := s.store.VerifyLogin(req.Username, req.Password)
	if !ok {
		s.limiter.Fail(keys...)
		s.record(audit.Event{Event: audit.LoginFailure, Subject: req.Username, IP: ip})
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	s.limiter.Reset(keys...)

	switch {
	case user.State == store.StatePending:
		secret, err := totp.NewSecret()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not start enrolment")
			return
		}
		value, err := s.store.PutPending(&store.Pending{Subject: user.Username, Stage: store.StageEnrol, TOTPSecret: secret})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not start enrolment")
			return
		}
		s.setPendingCookie(w, value)
		writeJSON(w, http.StatusOK, map[string]string{"next": "enrol"})

	case user.TOTPRequired:
		value, err := s.store.PutPending(&store.Pending{Subject: user.Username, Stage: store.StageTOTP})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not start the second factor")
			return
		}
		s.setPendingCookie(w, value)
		writeJSON(w, http.StatusOK, map[string]string{"next": "totp"})

	default:
		s.finish(w, r, user.Username, req.Redirect, ip)
	}
}

func (s *Server) finish(w http.ResponseWriter, r *http.Request, username, rd, ip string) {
	value, err := s.store.MintSession(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create a session")
		return
	}
	if cookie, err := r.Cookie(pendingCookie); err == nil {
		s.store.DeletePending(cookie.Value)
	}
	s.clearCookie(w, pendingCookie)
	s.setSessionCookie(w, value)
	s.record(audit.Event{Event: audit.LoginSuccess, Subject: username, IP: ip, Host: s.idp.Host})
	writeJSON(w, http.StatusOK, map[string]string{"next": "done", "redirect": s.resolveRedirect(rd)})
}

func (s *Server) pendingAt(r *http.Request, stage string) (*store.Pending, bool) {
	cookie, err := r.Cookie(pendingCookie)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	p, ok := s.store.Pending(cookie.Value)
	if !ok || p.Stage != stage {
		return nil, false
	}
	return p, true
}

func (s *Server) handleEnrolStart(w http.ResponseWriter, r *http.Request) {
	p, ok := s.pendingAt(r, store.StageEnrol)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no enrolment is in progress")
		return
	}
	secret := totp.Encode(p.TOTPSecret)
	writeJSON(w, http.StatusOK, map[string]string{
		"secret": secret,
		"uri":    totp.URI(s.cfg.Identity.Issuer, p.Subject, secret),
	})
}

func (s *Server) handleEnrolFinish(w http.ResponseWriter, r *http.Request) {
	p, ok := s.pendingAt(r, store.StageEnrol)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no enrolment is in progress")
		return
	}
	var req codeRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	ip := clientIP(r)
	key := "totp:" + p.Subject
	if s.limiter.Locked(key) {
		s.dropPending(w, r, p.Subject)
		writeError(w, http.StatusTooManyRequests, "too many failed codes, start again from your password")
		return
	}

	step, valid := totp.Validate(p.TOTPSecret, req.Code, time.Now().Unix(), 0)
	if !valid {
		s.limiter.Fail(key)
		s.record(audit.Event{Event: audit.TOTPFailure, Subject: p.Subject, IP: ip, Detail: "enrolment"})
		writeError(w, http.StatusBadRequest, "that code did not match, check your authenticator and try again")
		return
	}
	s.limiter.Reset(key)

	codes, err := s.store.EnrolTOTP(p.Subject, p.TOTPSecret, step)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not complete enrolment")
		return
	}
	s.record(audit.Event{Event: audit.TOTPEnrolled, Subject: p.Subject, IP: ip})

	value, err := s.store.MintSession(p.Subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create a session")
		return
	}
	if cookie, err := r.Cookie(pendingCookie); err == nil {
		s.store.DeletePending(cookie.Value)
	}
	s.clearCookie(w, pendingCookie)
	s.setSessionCookie(w, value)
	s.record(audit.Event{Event: audit.LoginSuccess, Subject: p.Subject, IP: ip, Host: s.idp.Host})
	writeJSON(w, http.StatusOK, map[string]any{
		"recovery_codes": codes,
		"redirect":       s.resolveRedirect(req.Redirect),
	})
}

func (s *Server) handleTOTPChallenge(w http.ResponseWriter, r *http.Request) {
	p, ok := s.pendingAt(r, store.StageTOTP)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no second factor is in progress")
		return
	}
	var req codeRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	ip := clientIP(r)
	key := "totp:" + p.Subject
	if s.limiter.Locked(key) {
		s.dropPending(w, r, p.Subject)
		writeError(w, http.StatusTooManyRequests, "too many failed codes, start again from your password")
		return
	}

	valid, wasRecovery := s.store.ConsumeTOTP(p.Subject, req.Code)
	if !valid {
		s.limiter.Fail(key)
		s.record(audit.Event{Event: audit.TOTPFailure, Subject: p.Subject, IP: ip})
		writeError(w, http.StatusUnauthorized, "that code did not match")
		return
	}
	s.limiter.Reset(key)
	if wasRecovery {
		s.record(audit.Event{Event: audit.RecoveryUsed, Subject: p.Subject, IP: ip})
	}
	s.finish(w, r, p.Subject, req.Redirect, ip)
}

func (s *Server) dropPending(w http.ResponseWriter, r *http.Request, subject string) {
	s.store.DeletePendingFor(subject)
	s.clearCookie(w, pendingCookie)
	s.record(audit.Event{Event: audit.LoginLocked, Subject: subject, IP: clientIP(r), Detail: "second factor"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		if p, err := s.store.ResolveSession(cookie.Value); err == nil {
			s.record(audit.Event{Event: audit.SessionRevoked, Subject: p.Subject, IP: clientIP(r), Detail: "logout"})
		}
		if err := s.store.RevokeSessionValue(cookie.Value); err != nil {
			log.Debug().Err(err).Msg("logout on an unknown session")
		}
	}
	s.clearCookie(w, sessionCookie)
	s.clearCookie(w, pendingCookie)
	w.WriteHeader(http.StatusNoContent)
}
