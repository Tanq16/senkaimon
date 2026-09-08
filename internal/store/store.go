package store

import (
	"cmp"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/Tanq16/senkaimon/internal/policy"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

type usersFile struct {
	Users []*User `json:"users"`
}

type tokensFile struct {
	Tokens []*Token `json:"tokens"`
}

type policiesFile struct {
	Policies []policy.Policy `json:"policies"`
}

type sessionsFile struct {
	Sessions []*Session `json:"sessions"`
}

type Store struct {
	mu  sync.RWMutex
	cfg *Config
	dir string

	users    map[string]*User
	tokens   map[string]*Token
	policies []policy.Policy
	sessions map[string]*Session
	pending  map[string]*Pending

	sessionsDirty bool
	tokensDirty   bool
}

func Initialize() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	empty := map[string]any{
		UsersFile:    usersFile{Users: []*User{}},
		TokensFile:   tokensFile{Tokens: []*Token{}},
		PoliciesFile: policiesFile{Policies: []policy.Policy{}},
		SessionsFile: sessionsFile{Sessions: []*Session{}},
	}
	for name, value := range empty {
		if err := writeJSON(filepath.Join(dir, name), value); err != nil {
			return err
		}
	}
	return nil
}

func Open(cfg *Config) (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	s := &Store{
		cfg:      cfg,
		dir:      dir,
		users:    map[string]*User{},
		tokens:   map[string]*Token{},
		sessions: map[string]*Session{},
		pending:  map[string]*Pending{},
	}

	var uf usersFile
	if err := readJSON(filepath.Join(dir, UsersFile), &uf); err != nil {
		return nil, err
	}
	for _, u := range uf.Users {
		s.users[u.Username] = u
	}

	var tf tokensFile
	if err := readJSON(filepath.Join(dir, TokensFile), &tf); err != nil {
		return nil, err
	}
	for _, t := range tf.Tokens {
		s.tokens[t.ID] = t
	}

	var pf policiesFile
	if err := readJSON(filepath.Join(dir, PoliciesFile), &pf); err != nil {
		return nil, err
	}
	s.policies = pf.Policies

	var sf sessionsFile
	if err := readJSON(filepath.Join(dir, SessionsFile), &sf); err != nil {
		return nil, err
	}
	for _, sess := range sf.Sessions {
		s.sessions[sess.Hash] = sess
	}
	return s, nil
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

func (s *Store) writeUsers() error {
	users := slices.SortedFunc(maps.Values(s.users), func(a, b *User) int {
		return cmp.Compare(a.Username, b.Username)
	})
	return writeJSON(filepath.Join(s.dir, UsersFile), usersFile{Users: users})
}

func (s *Store) writeTokens() error {
	tokens := slices.SortedFunc(maps.Values(s.tokens), func(a, b *Token) int {
		return cmp.Compare(a.ID, b.ID)
	})
	s.tokensDirty = false
	return writeJSON(filepath.Join(s.dir, TokensFile), tokensFile{Tokens: tokens})
}

func (s *Store) writePolicies() error {
	return writeJSON(filepath.Join(s.dir, PoliciesFile), policiesFile{Policies: s.policies})
}

func (s *Store) writeSessions() error {
	sessions := slices.SortedFunc(maps.Values(s.sessions), func(a, b *Session) int {
		return cmp.Compare(a.Hash, b.Hash)
	})
	s.sessionsDirty = false
	return writeJSON(filepath.Join(s.dir, SessionsFile), sessionsFile{Sessions: sessions})
}
