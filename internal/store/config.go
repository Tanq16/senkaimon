package store

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	ConfigFile   = "config.yaml"
	UsersFile    = "users.json"
	TokensFile   = "tokens.json"
	PoliciesFile = "policies.json"
	SessionsFile = "sessions.json"
	AuditFile    = "audit.log"
)

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type IdentityConfig struct {
	Issuer       string `yaml:"issuer"`
	IDPURL       string `yaml:"idp_url"`
	CookieDomain string `yaml:"cookie_domain"`
}

type SessionConfig struct {
	IdleTTL       time.Duration `yaml:"idle_ttl"`
	AbsoluteTTL   time.Duration `yaml:"absolute_ttl"`
	PendingTTL    time.Duration `yaml:"pending_ttl"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

type Argon2Config struct {
	MemoryKiB uint32 `yaml:"memory_kib"`
	Time      uint32 `yaml:"time"`
	Threads   uint8  `yaml:"threads"`
}

type RateLimitConfig struct {
	MaxFailures int           `yaml:"max_failures"`
	Window      time.Duration `yaml:"window"`
	Lockout     time.Duration `yaml:"lockout"`
}

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Identity  IdentityConfig  `yaml:"identity"`
	Session   SessionConfig   `yaml:"session"`
	Argon2    Argon2Config    `yaml:"argon2"`
	RateLimit RateLimitConfig `yaml:"ratelimit"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "senkaimon"), nil
}

func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func LoadConfig() (*Config, error) {
	path, err := Path(ConfigFile)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s does not exist, run 'senkaimon setup' first", path)
	}
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func WriteConfig(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, ConfigFile), data)
}

func (c *Config) Validate() error {
	if c.Server.Listen == "" {
		return errors.New("server.listen is empty")
	}
	if c.Identity.Issuer == "" {
		return errors.New("identity.issuer is empty")
	}
	if c.Identity.CookieDomain == "" {
		return errors.New("identity.cookie_domain is empty")
	}
	if _, err := ParseIDPURL(c.Identity.IDPURL); err != nil {
		return err
	}
	if c.Session.IdleTTL <= 0 || c.Session.AbsoluteTTL <= 0 || c.Session.PendingTTL <= 0 || c.Session.FlushInterval <= 0 {
		return errors.New("every session ttl must be a positive duration")
	}
	if c.Argon2.MemoryKiB == 0 || c.Argon2.Time == 0 || c.Argon2.Threads == 0 {
		return errors.New("every argon2 parameter must be non-zero")
	}
	if c.RateLimit.MaxFailures <= 0 || c.RateLimit.Window <= 0 || c.RateLimit.Lockout <= 0 {
		return errors.New("every ratelimit parameter must be positive")
	}
	return nil
}

func ParseIDPURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("identity.idp_url is not a URL: %w", err)
	}
	if u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("identity.idp_url must be an absolute https URL, got %q", raw)
	}
	return u, nil
}

func CookieDomainFor(domain string) string {
	return "." + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

func DefaultConfig() *Config {
	return &Config{
		Server:   ServerConfig{Listen: "127.0.0.1:4180"},
		Identity: IdentityConfig{Issuer: "Senkaimon"},
		Session: SessionConfig{
			IdleTTL:       24 * time.Hour,
			AbsoluteTTL:   720 * time.Hour,
			PendingTTL:    5 * time.Minute,
			FlushInterval: 60 * time.Second,
		},
		Argon2:    Argon2Config{MemoryKiB: 65536, Time: 3, Threads: 4},
		RateLimit: RateLimitConfig{MaxFailures: 5, Window: 15 * time.Minute, Lockout: 15 * time.Minute},
	}
}
