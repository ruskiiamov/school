package auth

import (
	"sync"
	"time"
)

const (
	maxTrackedAttempts = 10000
	maxAttemptKeyBytes = 64
)

var (
	loginLimit  = attemptLimit{failures: 5, window: 15 * time.Minute, block: 5 * time.Minute}
	deviceLimit = attemptLimit{failures: 5, window: 15 * time.Minute, block: 5 * time.Minute}
	ipLimit     = attemptLimit{failures: 30, window: 5 * time.Minute, block: 5 * time.Minute}
)

type attemptLimit struct {
	failures int
	window   time.Duration
	block    time.Duration
}

type attemptEntry struct {
	count        int
	started      time.Time
	blockedUntil time.Time
}

type attemptLimiter struct {
	mu      sync.Mutex
	limit   attemptLimit
	now     func() time.Time
	entries map[string]attemptEntry
}

func newAttemptLimiter(limit attemptLimit) *attemptLimiter {
	return &attemptLimiter{limit: limit, now: time.Now, entries: make(map[string]attemptEntry)}
}

func (a *attemptLimiter) allow(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	key = attemptKey(key)

	entry, tracked := a.entries[key]
	if now.Before(entry.blockedUntil) {
		return false
	}

	if !tracked && !a.makeRoom(now) {
		return true
	}

	if !tracked || a.expired(entry, now) {
		entry = attemptEntry{started: now}
	}

	entry.count++

	if entry.count >= a.limit.failures {
		entry.blockedUntil = now.Add(a.limit.block)
	}

	a.entries[key] = entry

	return true
}

func (a *attemptLimiter) refund(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key = attemptKey(key)

	entry, tracked := a.entries[key]
	if !tracked {
		return
	}

	entry.count--

	if entry.count <= 0 {
		delete(a.entries, key)
		return
	}

	if entry.count < a.limit.failures {
		entry.blockedUntil = time.Time{}
	}

	a.entries[key] = entry
}

func (a *attemptLimiter) reset(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.entries, attemptKey(key))
}

func (a *attemptLimiter) expired(entry attemptEntry, now time.Time) bool {
	return now.Sub(entry.started) >= a.limit.window && !now.Before(entry.blockedUntil)
}

func (a *attemptLimiter) makeRoom(now time.Time) bool {
	if len(a.entries) < maxTrackedAttempts {
		return true
	}

	for key, entry := range a.entries {
		if a.expired(entry, now) {
			delete(a.entries, key)
		}
	}

	if len(a.entries) < maxTrackedAttempts {
		return true
	}

	for key, entry := range a.entries {
		if !now.Before(entry.blockedUntil) {
			delete(a.entries, key)
		}
	}

	return len(a.entries) < maxTrackedAttempts
}

func attemptKey(key string) string {
	if len(key) > maxAttemptKeyBytes {
		return key[:maxAttemptKeyBytes]
	}

	return key
}
