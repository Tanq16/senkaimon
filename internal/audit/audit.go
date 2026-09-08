package audit

import (
	"bufio"
	"encoding/json/v2"
	"os"
	"slices"
	"sync"
	"time"
)

const (
	LoginSuccess   = "login.success"
	LoginFailure   = "login.failure"
	LoginLocked    = "login.locked"
	TOTPFailure    = "totp.failure"
	TOTPEnrolled   = "totp.enrolled"
	RecoveryUsed   = "recovery.used"
	SessionRevoked = "session.revoked"
	TokenMinted    = "token.minted"
	TokenRevoked   = "token.revoked"
	TokenDenied    = "token.denied"
	UserCreated    = "user.created"
	UserUpdated    = "user.updated"
	UserDeleted    = "user.deleted"
	PolicyUpdated  = "policy.updated"
	AccessDenied   = "access.denied"
)

type Event struct {
	TS      time.Time `json:"ts"`
	Event   string    `json:"event"`
	Subject string    `json:"subject,omitzero"`
	IP      string    `json:"ip,omitzero"`
	Host    string    `json:"host,omitzero"`
	Detail  string    `json:"detail,omitzero"`
}

type Log struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func Open(path string) (*Log, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Log{path: path, file: f}, nil
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

func (l *Log) Write(e Event) error {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.file.Write(append(line, '\n')); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *Log) Tail(limit int) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if limit <= 0 {
		limit = 1
	}
	events := make([]Event, 0, limit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if len(events) == limit {
			events = slices.Delete(events, 0, 1)
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	slices.Reverse(events)
	return events, nil
}
