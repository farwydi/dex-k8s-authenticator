package main

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// loginSessionTTL bounds how long a generated state/verifier pair stays
// usable. Authorization redirects normally complete within seconds; this
// cap keeps the in-memory store from growing without bound if a user
// abandons a login.
const loginSessionTTL = 10 * time.Minute

// loginSession ties a generated PKCE code_verifier to a state value so the
// callback handler can complete the token exchange. expires bounds the
// lifetime of the entry.
type loginSession struct {
	verifier string
	expires  time.Time
}

// loginSessionStore keeps per-state PKCE verifiers in memory. Entries are
// single-use: a successful (or failed) lookup removes the entry. A periodic
// sweep evicts abandoned entries.
type loginSessionStore struct {
	mu       sync.Mutex
	sessions map[string]loginSession
	now      func() time.Time
}

func newLoginSessionStore() *loginSessionStore {
	return &loginSessionStore{
		sessions: make(map[string]loginSession),
		now:      time.Now,
	}
}

// put records verifier under state with the default TTL.
func (s *loginSessionStore) put(state, verifier string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.sessions[state] = loginSession{
		verifier: verifier,
		expires:  s.now().Add(loginSessionTTL),
	}
}

// take returns the verifier registered for state and removes the entry.
// The second return value is false when no live entry exists.
func (s *loginSessionStore) take(state string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[state]
	if !ok {
		return "", false
	}
	delete(s.sessions, state)
	if s.now().After(entry.expires) {
		return "", false
	}
	return entry.verifier, true
}

func (s *loginSessionStore) sweepLocked() {
	now := s.now()
	for k, v := range s.sessions {
		if now.After(v.expires) {
			delete(s.sessions, k)
		}
	}
}

// generateState returns a 32-byte URL-safe random string suitable for use
// as the OAuth 2.0 state parameter.
func generateState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
