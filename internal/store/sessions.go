package store

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"time"
)

func (s *Store) MintSession(subject string) (string, error) {
	value, hash, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[hash] = &Session{
		Hash:      hash,
		Subject:   subject,
		IssuedAt:  now,
		LastSeen:  now,
		ExpiresAt: now.Add(s.cfg.Session.AbsoluteTTL),
	}
	if err := s.writeSessions(); err != nil {
		delete(s.sessions, hash)
		return "", err
	}
	return value, nil
}

func (s *Store) ResolveSession(value string) (Principal, error) {
	hash := sha256Hex(value)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[hash]
	if !ok {
		return Principal{}, ErrNotFound
	}
	if now.After(sess.ExpiresAt) || now.After(sess.LastSeen.Add(s.cfg.Session.IdleTTL)) {
		delete(s.sessions, hash)
		s.sessionsDirty = true
		return Principal{}, errors.New("session has expired")
	}
	u, ok := s.users[sess.Subject]
	if !ok {
		delete(s.sessions, hash)
		s.sessionsDirty = true
		return Principal{}, ErrNotFound
	}
	sess.LastSeen = now.UTC()
	s.sessionsDirty = true
	return Principal{Kind: KindUser, Subject: u.Username, Owner: u.Username, Admin: u.Admin}, nil
}

func (s *Store) Sessions() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]Session, 0, len(s.sessions))
	for _, sess := range slices.SortedFunc(maps.Values(s.sessions), func(a, b *Session) int {
		return cmp.Compare(b.IssuedAt.UnixNano(), a.IssuedAt.UnixNano())
	}) {
		sessions = append(sessions, *sess)
	}
	return sessions
}

func (s *Store) RevokeSession(hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[hash]; !ok {
		return ErrNotFound
	}
	delete(s.sessions, hash)
	return s.writeSessions()
}

func (s *Store) RevokeSessionValue(value string) error {
	return s.RevokeSession(sha256Hex(value))
}

func (s *Store) PutPending(p *Pending) (string, error) {
	value, hash, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ExpiresAt = time.Now().Add(s.cfg.Session.PendingTTL)
	s.pending[hash] = p
	return value, nil
}

func (s *Store) Pending(value string) (*Pending, bool) {
	hash := sha256Hex(value)
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[hash]
	if !ok {
		return nil, false
	}
	if time.Now().After(p.ExpiresAt) {
		delete(s.pending, hash)
		return nil, false
	}
	return p, true
}

func (s *Store) DeletePending(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, sha256Hex(value))
}

func (s *Store) DeletePendingFor(subject string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	maps.DeleteFunc(s.pending, func(_ string, p *Pending) bool { return p.Subject == subject })
}

func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionsDirty {
		if err := s.writeSessions(); err != nil {
			return err
		}
	}
	if s.tokensDirty {
		if err := s.writeTokens(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Sweep() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	before := len(s.sessions)
	maps.DeleteFunc(s.sessions, func(_ string, sess *Session) bool {
		return now.After(sess.ExpiresAt) || now.After(sess.LastSeen.Add(s.cfg.Session.IdleTTL))
	})
	maps.DeleteFunc(s.pending, func(_ string, p *Pending) bool { return now.After(p.ExpiresAt) })
	if len(s.sessions) != before {
		s.sessionsDirty = true
	}
}
