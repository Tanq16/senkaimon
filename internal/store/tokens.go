package store

import (
	"cmp"
	"crypto/subtle"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"
)

func (s *Store) Tokens() []Token {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tokens := make([]Token, 0, len(s.tokens))
	for _, t := range slices.SortedFunc(maps.Values(s.tokens), func(a, b *Token) int { return cmp.Compare(a.ID, b.ID) }) {
		tokens = append(tokens, *t)
	}
	return tokens
}

func (s *Store) MintToken(name, owner string, expiresAt *time.Time) (Token, string, error) {
	if name == "" {
		return Token{}, "", errors.New("token name is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[owner]; !ok {
		return Token{}, "", fmt.Errorf("owner %q %w", owner, ErrNotFound)
	}

	var id string
	for range 8 {
		candidate, err := randomID()
		if err != nil {
			return Token{}, "", err
		}
		if _, taken := s.tokens[candidate]; taken {
			continue
		}
		if _, taken := s.users[candidate]; taken {
			continue
		}
		id = candidate
		break
	}
	if id == "" {
		return Token{}, "", errors.New("could not allocate a free token id")
	}

	full, hash, err := buildToken(id)
	if err != nil {
		return Token{}, "", err
	}
	t := &Token{
		ID:        id,
		Name:      name,
		Owner:     owner,
		Hash:      hash,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	}
	s.tokens[id] = t
	if err := s.writeTokens(); err != nil {
		delete(s.tokens, id)
		return Token{}, "", err
	}
	return *t, full, nil
}

func (s *Store) RevokeToken(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tokens[id]; !ok {
		return fmt.Errorf("token %q %w", id, ErrNotFound)
	}
	delete(s.tokens, id)
	s.dropSubjects([]string{id})
	if err := s.writeTokens(); err != nil {
		return err
	}
	return s.writePolicies()
}

func (s *Store) ResolveToken(presented string) (Principal, error) {
	id, hash, err := ParseToken(presented)
	if err != nil {
		return Principal{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[id]
	if !ok {
		return Principal{}, ErrNotFound
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(t.Hash)) != 1 {
		return Principal{}, ErrNotFound
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return Principal{}, errors.New("token has expired")
	}
	t.LastUsed = time.Now().UTC()
	s.tokensDirty = true
	return Principal{Kind: KindToken, Subject: t.ID, Owner: t.Owner}, nil
}
