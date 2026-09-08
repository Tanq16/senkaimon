package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/policy"
	"github.com/Tanq16/senkaimon/internal/store"
)

type userView struct {
	Username      string    `json:"username"`
	Admin         bool      `json:"admin"`
	State         string    `json:"state"`
	TOTPRequired  bool      `json:"totp_required"`
	RecoveryCodes int       `json:"recovery_codes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type tokenView struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Owner     string     `json:"owner"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	LastUsed  time.Time  `json:"last_used,omitzero"`
}

type sessionView struct {
	Hash      string    `json:"hash"`
	Subject   string    `json:"subject"`
	IssuedAt  time.Time `json:"issued_at"`
	LastSeen  time.Time `json:"last_seen"`
	ExpiresAt time.Time `json:"expires_at"`
}

func viewUser(u store.User) userView {
	return userView{
		Username:      u.Username,
		Admin:         u.Admin,
		State:         u.State,
		TOTPRequired:  u.TOTPRequired,
		RecoveryCodes: len(u.RecoveryCodes),
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func viewToken(t store.Token) tokenView {
	return tokenView{ID: t.ID, Name: t.Name, Owner: t.Owner, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsed: t.LastUsed}
}

func viewSession(s store.Session) sessionView {
	return sessionView{Hash: s.Hash, Subject: s.Subject, IssuedAt: s.IssuedAt, LastSeen: s.LastSeen, ExpiresAt: s.ExpiresAt}
}

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request) {
	users := s.store.Users()
	out := make([]userView, 0, len(users))
	for _, u := range users {
		out = append(out, viewUser(u))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		Admin        bool   `json:"admin"`
		TOTPRequired bool   `json:"totp_required"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	if err := s.store.CreateUser(req.Username, req.Password, req.Admin, req.TOTPRequired); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.UserCreated, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: req.Username})
	u, _ := s.store.User(req.Username)
	writeJSON(w, http.StatusCreated, viewUser(u))
}

func (s *Server) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	var req struct {
		Admin        *bool   `json:"admin"`
		TOTPRequired *bool   `json:"totp_required"`
		Password     *string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	me := principalOf(r)
	if username == me.Subject && req.Admin != nil && !*req.Admin {
		writeError(w, http.StatusBadRequest, "an admin cannot demote their own account")
		return
	}
	if err := s.store.UpdateUser(username, store.UserUpdate{Admin: req.Admin, TOTPRequired: req.TOTPRequired, Password: req.Password}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.UserUpdated, Subject: me.Subject, IP: clientIP(r), Detail: username})
	u, _ := s.store.User(username)
	writeJSON(w, http.StatusOK, viewUser(u))
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	me := principalOf(r)
	if username == me.Subject {
		writeError(w, http.StatusBadRequest, "an admin cannot delete their own account")
		return
	}
	if err := s.store.DeleteUser(username); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.UserDeleted, Subject: me.Subject, IP: clientIP(r), Detail: username})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUserResetTOTP(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if err := s.store.ResetTOTP(username); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.store.DeletePendingFor(username)
	s.record(audit.Event{Event: audit.UserUpdated, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: "reset totp for " + username})
	u, _ := s.store.User(username)
	writeJSON(w, http.StatusOK, viewUser(u))
}

func (s *Server) handleTokensList(w http.ResponseWriter, r *http.Request) {
	tokens := s.store.Tokens()
	out := make([]tokenView, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, viewToken(t))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleTokenMint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string     `json:"name"`
		Owner     string     `json:"owner"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	token, full, err := s.store.MintToken(req.Name, req.Owner, req.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.TokenMinted, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: token.ID})
	writeJSON(w, http.StatusCreated, map[string]any{"token": full, "record": viewToken(token)})
}

func (s *Server) handleTokenRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.RevokeToken(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.TokenRevoked, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePoliciesList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Policies())
}

func (s *Server) handlePolicyPut(w http.ResponseWriter, r *http.Request) {
	var p policy.Policy
	if err := readJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	p.Name = r.PathValue("name")
	if err := s.store.PutPolicy(p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.PolicyUpdated, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: p.Name})
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.DeletePolicy(name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.PolicyUpdated, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: "deleted " + name})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request) {
	sessions := s.store.Sessions()
	out := make([]sessionView, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, viewSession(sess))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSessionRevoke(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if err := s.store.RevokeSession(hash); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.record(audit.Event{Event: audit.SessionRevoked, Subject: principalOf(r).Subject, IP: clientIP(r), Detail: hash})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(parsed, 5000)
	}
	events, err := s.audit.Tail(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the audit log")
		return
	}
	writeJSON(w, http.StatusOK, events)
}
