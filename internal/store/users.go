package store

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Tanq16/senkaimon/internal/totp"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{1,32}$`)

const MinPasswordLength = 12

func ValidateUsername(name string) error {
	if !usernamePattern.MatchString(name) {
		return errors.New("username must be 1 to 32 characters of a-z, 0-9, dot, underscore or hyphen")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	return nil
}

func (s *Store) Users() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]User, 0, len(s.users))
	for _, u := range slices.SortedFunc(maps.Values(s.users), func(a, b *User) int { return strings.Compare(a.Username, b.Username) }) {
		users = append(users, *u)
	}
	return users
}

func (s *Store) User(username string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return User{}, false
	}
	return *u, true
}

func (s *Store) CreateUser(username, password string, admin, totpRequired bool) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[username]; exists {
		return fmt.Errorf("user %q %w", username, ErrAlreadyExists)
	}
	if _, exists := s.tokens[username]; exists {
		return fmt.Errorf("%q collides with an existing token id", username)
	}

	hash, err := HashPassword(password, s.cfg.Argon2)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	state := StateActive
	if totpRequired {
		state = StatePending
	}
	s.users[username] = &User{
		Username:     username,
		Admin:        admin,
		State:        state,
		Password:     hash,
		TOTPRequired: totpRequired,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.writeUsers()
}

type UserUpdate struct {
	Admin        *bool
	TOTPRequired *bool
	Password     *string
}

func (s *Store) UpdateUser(username string, up UserUpdate) error {
	if up.Password != nil {
		if err := ValidatePassword(*up.Password); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		return fmt.Errorf("user %q %w", username, ErrNotFound)
	}

	if up.Admin != nil {
		u.Admin = *up.Admin
	}
	if up.Password != nil {
		hash, err := HashPassword(*up.Password, s.cfg.Argon2)
		if err != nil {
			return err
		}
		u.Password = hash
	}
	if up.TOTPRequired != nil && *up.TOTPRequired != u.TOTPRequired {
		u.TOTPRequired = *up.TOTPRequired
		if u.TOTPRequired {
			s.clearTOTP(u)
		} else {
			u.State = StateActive
			u.TOTPSecret = ""
			u.TOTPLastStep = 0
			u.RecoveryCodes = nil
		}
	}
	u.UpdatedAt = time.Now().UTC()
	return s.writeUsers()
}

func (s *Store) clearTOTP(u *User) {
	u.State = StatePending
	u.TOTPSecret = ""
	u.TOTPLastStep = 0
	u.RecoveryCodes = nil
}

func (s *Store) ResetTOTP(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		return fmt.Errorf("user %q %w", username, ErrNotFound)
	}
	u.TOTPRequired = true
	s.clearTOTP(u)
	u.UpdatedAt = time.Now().UTC()
	return s.writeUsers()
}

func (s *Store) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; !ok {
		return fmt.Errorf("user %q %w", username, ErrNotFound)
	}
	delete(s.users, username)

	subjects := []string{username}
	for id, t := range s.tokens {
		if t.Owner == username {
			delete(s.tokens, id)
			subjects = append(subjects, id)
		}
	}
	maps.DeleteFunc(s.sessions, func(_ string, sess *Session) bool { return sess.Subject == username })
	maps.DeleteFunc(s.pending, func(_ string, p *Pending) bool { return p.Subject == username })
	s.dropSubjects(subjects)

	if err := s.writeUsers(); err != nil {
		return err
	}
	if err := s.writeTokens(); err != nil {
		return err
	}
	if err := s.writePolicies(); err != nil {
		return err
	}
	return s.writeSessions()
}

func (s *Store) dropSubjects(subjects []string) {
	for i := range s.policies {
		s.policies[i].Subjects = slices.DeleteFunc(s.policies[i].Subjects, func(sub string) bool {
			return slices.Contains(subjects, sub)
		})
	}
}

func (s *Store) VerifyLogin(username, password string) (User, bool) {
	s.mu.RLock()
	u, ok := s.users[username]
	if !ok {
		s.mu.RUnlock()
		return User{}, false
	}
	stored := u.Password
	s.mu.RUnlock()

	if !VerifyPassword(password, stored) {
		return User{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok = s.users[username]
	if !ok {
		return User{}, false
	}
	if !u.Password.Matches(s.cfg.Argon2) {
		if hash, err := HashPassword(password, s.cfg.Argon2); err == nil {
			u.Password = hash
			u.UpdatedAt = time.Now().UTC()
			_ = s.writeUsers()
		}
	}
	return *u, true
}

func (s *Store) EnrolTOTP(username string, secret []byte, step uint64) ([]string, error) {
	plain, hashed, err := NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		return nil, fmt.Errorf("user %q %w", username, ErrNotFound)
	}
	u.State = StateActive
	u.TOTPRequired = true
	u.TOTPSecret = totp.Encode(secret)
	u.TOTPLastStep = step
	u.RecoveryCodes = hashed
	u.UpdatedAt = time.Now().UTC()
	if err := s.writeUsers(); err != nil {
		return nil, err
	}
	return plain, nil
}

func (s *Store) ConsumeTOTP(username, code string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok {
		return false, false
	}

	if secret, err := totp.Decode(u.TOTPSecret); err == nil && len(secret) > 0 {
		if step, valid := totp.Validate(secret, code, time.Now().Unix(), u.TOTPLastStep); valid {
			u.TOTPLastStep = step
			u.UpdatedAt = time.Now().UTC()
			return s.writeUsers() == nil, false
		}
	}

	wanted := sha256Hex(NormalizeRecoveryCode(code))
	idx := slices.Index(u.RecoveryCodes, wanted)
	if idx < 0 {
		return false, false
	}
	u.RecoveryCodes = slices.Delete(u.RecoveryCodes, idx, idx+1)
	u.UpdatedAt = time.Now().UTC()
	if err := s.writeUsers(); err != nil {
		return false, false
	}
	return true, true
}
