package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const TokenPrefix = "senkaimon_"

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func HashPassword(password string, p Argon2Config) (PasswordHash, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return PasswordHash{}, err
	}
	sum := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, 32)
	return PasswordHash{
		Algo:      "argon2id",
		MemoryKiB: p.MemoryKiB,
		Time:      p.Time,
		Threads:   p.Threads,
		Salt:      base64.StdEncoding.EncodeToString(salt),
		Hash:      base64.StdEncoding.EncodeToString(sum),
	}, nil
}

func VerifyPassword(password string, h PasswordHash) bool {
	salt, err := base64.StdEncoding.DecodeString(h.Salt)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(h.Hash)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, h.Time, h.MemoryKiB, h.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (h PasswordHash) Matches(p Argon2Config) bool {
	return h.Algo == "argon2id" && h.MemoryKiB == p.MemoryKiB && h.Time == p.Time && h.Threads == p.Threads
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomID() (string, error) {
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return strings.ToLower(idEncoding.EncodeToString(raw)), nil
}

func NewSessionToken() (value, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	value = base64.RawURLEncoding.EncodeToString(raw)
	return value, sha256Hex(value), nil
}

func NewRecoveryCodes() (plain, hashed []string, err error) {
	for range 10 {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		code := idEncoding.EncodeToString(raw)
		plain = append(plain, code)
		hashed = append(hashed, sha256Hex(code))
	}
	return plain, hashed, nil
}

func NormalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code)))
}

func ParseToken(presented string) (id, hash string, err error) {
	body, ok := strings.CutPrefix(presented, TokenPrefix)
	if !ok {
		return "", "", errors.New("token does not carry the senkaimon prefix")
	}
	id, _, found := strings.Cut(body, "_")
	if !found || id == "" {
		return "", "", errors.New("token carries no id")
	}
	return id, sha256Hex(presented), nil
}

func buildToken(id string) (full, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	full = fmt.Sprintf("%s%s_%s", TokenPrefix, id, base64.RawURLEncoding.EncodeToString(raw))
	return full, sha256Hex(full), nil
}
