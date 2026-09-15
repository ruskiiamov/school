package server

import (
	"net/http"
	"sync"
	"time"
)

const credentialsTTL = 10 * time.Minute

type credentialsKind int

const (
	credentialsCreated credentialsKind = iota
	credentialsPasswordChanged
)

type credentialsEntry struct {
	userID   int64
	kind     credentialsKind
	login    string
	password string
	expires  time.Time
}

type credentialsStore struct {
	mu      sync.Mutex
	entries map[string]credentialsEntry
}

func newCredentialsStore() *credentialsStore {
	return &credentialsStore{entries: make(map[string]credentialsEntry)}
}

func (c *credentialsStore) put(sessionID string, entry credentialsEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.purge(now)

	entry.expires = now.Add(credentialsTTL)
	c.entries[sessionID] = entry
}

func (c *credentialsStore) take(sessionID string, userID int64, kind credentialsKind) (credentialsEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purge(time.Now())

	entry, ok := c.entries[sessionID]
	if !ok || entry.userID != userID || entry.kind != kind {
		return credentialsEntry{}, false
	}

	delete(c.entries, sessionID)

	return entry, true
}

func (c *credentialsStore) purge(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expires) {
			delete(c.entries, key)
		}
	}
}

func (s *Server) sessionID(r *http.Request) string {
	cookie, err := r.Cookie(s.cookie.CookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}
