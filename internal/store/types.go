package store

import "time"

type Kind string

const (
	KindUser  Kind = "user"
	KindToken Kind = "token"
)

const (
	StatePending = "pending"
	StateActive  = "active"
)

type Principal struct {
	Kind    Kind
	Subject string
	Owner   string
	Admin   bool
}

type PasswordHash struct {
	Algo      string `json:"algo"`
	MemoryKiB uint32 `json:"memory_kib"`
	Time      uint32 `json:"time"`
	Threads   uint8  `json:"threads"`
	Salt      string `json:"salt"`
	Hash      string `json:"hash"`
}

type User struct {
	Username      string       `json:"username"`
	Admin         bool         `json:"admin"`
	State         string       `json:"state"`
	Password      PasswordHash `json:"password"`
	TOTPRequired  bool         `json:"totp_required"`
	TOTPSecret    string       `json:"totp_secret,omitzero"`
	TOTPLastStep  uint64       `json:"totp_last_step,omitzero"`
	RecoveryCodes []string     `json:"recovery_codes,omitzero"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Token struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Owner     string     `json:"owner"`
	Hash      string     `json:"hash"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	LastUsed  time.Time  `json:"last_used,omitzero"`
}

type Session struct {
	Hash      string    `json:"hash"`
	Subject   string    `json:"subject"`
	IssuedAt  time.Time `json:"issued_at"`
	LastSeen  time.Time `json:"last_seen"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Pending struct {
	Subject    string
	Stage      string
	TOTPSecret []byte
	ExpiresAt  time.Time
}

const (
	StageTOTP  = "totp"
	StageEnrol = "enrol"
)
