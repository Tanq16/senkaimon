package server

import (
	"net/http"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/store"
)

func totpState(u store.User) string {
	switch {
	case !u.TOTPRequired:
		return "off"
	case u.State == store.StatePending:
		return "pending"
	default:
		return "enrolled"
	}
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	me := principalOf(r)
	u, ok := s.store.User(me.Subject)
	if !ok {
		writeError(w, http.StatusNotFound, "account no longer exists")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":       u.Username,
		"admin":          u.Admin,
		"totp":           totpState(u),
		"recovery_codes": len(u.RecoveryCodes),
	})
}

func (s *Server) handleAccountPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	me := principalOf(r)
	if _, ok := s.store.VerifyLogin(me.Subject, req.CurrentPassword); !ok {
		writeError(w, http.StatusUnauthorized, "the current password is wrong")
		return
	}
	if err := s.store.UpdateUser(me.Subject, store.UserUpdate{Password: &req.NewPassword}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.UserUpdated, Subject: me.Subject, IP: clientIP(r), Detail: "changed own password"})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAccountSessions(w http.ResponseWriter, r *http.Request) {
	me := principalOf(r)
	out := make([]sessionView, 0)
	for _, sess := range s.store.Sessions() {
		if sess.Subject == me.Subject {
			out = append(out, viewSession(sess))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAccountSessionRevoke(w http.ResponseWriter, r *http.Request) {
	me := principalOf(r)
	hash := r.PathValue("hash")
	owned := false
	for _, sess := range s.store.Sessions() {
		if sess.Hash == hash && sess.Subject == me.Subject {
			owned = true
			break
		}
	}
	if !owned {
		writeError(w, http.StatusNotFound, "no such session")
		return
	}
	if err := s.store.RevokeSession(hash); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.SessionRevoked, Subject: me.Subject, IP: clientIP(r), Detail: hash})
	w.WriteHeader(http.StatusNoContent)
}
