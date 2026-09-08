package store

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Tanq16/senkaimon/internal/policy"
)

func (s *Store) Policies() []policy.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.policies)
}

func (s *Store) ValidatePolicy(p policy.Policy) error {
	if p.Name == "" {
		return errors.New("policy name is empty")
	}
	if len(p.Rules) == 0 {
		return errors.New("a policy must carry at least one rule")
	}
	for _, r := range p.Rules {
		if r.Effect != policy.EffectAllow && r.Effect != policy.EffectDeny {
			return fmt.Errorf("rule effect must be allow or deny, got %q", r.Effect)
		}
		if r.Host == "" {
			return errors.New("a rule host glob is empty")
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sub := range p.Subjects {
		_, isUser := s.users[sub]
		_, isToken := s.tokens[sub]
		if !isUser && !isToken {
			return fmt.Errorf("subject %q is neither a user nor a token id", sub)
		}
	}
	return nil
}

func (s *Store) PutPolicy(p policy.Policy) error {
	if err := s.ValidatePolicy(p); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := slices.IndexFunc(s.policies, func(existing policy.Policy) bool { return existing.Name == p.Name })
	if idx < 0 {
		s.policies = append(s.policies, p)
	} else {
		s.policies[idx] = p
	}
	return s.writePolicies()
}

func (s *Store) DeletePolicy(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := slices.IndexFunc(s.policies, func(p policy.Policy) bool { return p.Name == name })
	if idx < 0 {
		return fmt.Errorf("policy %q %w", name, ErrNotFound)
	}
	s.policies = slices.Delete(s.policies, idx, idx+1)
	return s.writePolicies()
}
