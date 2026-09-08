package ratelimit

import (
	"maps"
	"sync"
	"time"
)

type window struct {
	count     int
	startedAt time.Time
}

type Limiter struct {
	mu          sync.Mutex
	windows     map[string]*window
	maxFailures int
	span        time.Duration
	lockout     time.Duration
}

func New(maxFailures int, span, lockout time.Duration) *Limiter {
	return &Limiter{
		windows:     map[string]*window{},
		maxFailures: maxFailures,
		span:        span,
		lockout:     lockout,
	}
}

func (l *Limiter) stale(w *window, now time.Time) bool {
	return now.After(w.startedAt.Add(l.span)) && now.After(w.startedAt.Add(l.lockout))
}

func (l *Limiter) Locked(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, key := range keys {
		w, ok := l.windows[key]
		if !ok {
			continue
		}
		if l.stale(w, now) {
			delete(l.windows, key)
			continue
		}
		if w.count >= l.maxFailures && now.Before(w.startedAt.Add(l.lockout)) {
			return true
		}
	}
	return false
}

func (l *Limiter) Fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, key := range keys {
		w, ok := l.windows[key]
		if !ok || now.After(w.startedAt.Add(l.span)) {
			l.windows[key] = &window{count: 1, startedAt: now}
			continue
		}
		w.count++
	}
}

func (l *Limiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		delete(l.windows, key)
	}
}

func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	maps.DeleteFunc(l.windows, func(_ string, w *window) bool { return l.stale(w, now) })
}
