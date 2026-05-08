package main

import (
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestLoginSessionStore_RoundTrip(t *testing.T) {
	store := newLoginSessionStore()

	stateA, stateB := generateState(), generateState()
	if stateA == stateB {
		t.Fatalf("generateState produced identical values across calls")
	}

	verifierA := oauth2.GenerateVerifier()
	verifierB := oauth2.GenerateVerifier()
	if verifierA == verifierB {
		t.Fatalf("oauth2.GenerateVerifier produced identical values across calls")
	}

	store.put(stateA, verifierA)
	store.put(stateB, verifierB)

	if got, ok := store.take(stateA); !ok || got != verifierA {
		t.Fatalf("take(stateA) = %q, %v; want %q, true", got, ok, verifierA)
	}
	// take is single-use: a second lookup must miss.
	if got, ok := store.take(stateA); ok {
		t.Fatalf("take(stateA) after first take = %q, %v; want \"\", false", got, ok)
	}
	if got, ok := store.take(stateB); !ok || got != verifierB {
		t.Fatalf("take(stateB) = %q, %v; want %q, true", got, ok, verifierB)
	}
}

func TestLoginSessionStore_UnknownState(t *testing.T) {
	store := newLoginSessionStore()
	if got, ok := store.take("never-stored"); ok {
		t.Fatalf("take(unknown) = %q, %v; want \"\", false", got, ok)
	}
}

func TestLoginSessionStore_Expiry(t *testing.T) {
	store := newLoginSessionStore()
	now := time.Unix(1_700_000_000, 0)
	store.now = func() time.Time { return now }

	store.put("s1", "v1")

	// Within TTL: usable.
	now = now.Add(loginSessionTTL - time.Second)
	if got, ok := store.take("s1"); !ok || got != "v1" {
		t.Fatalf("within TTL: take = %q, %v; want \"v1\", true", got, ok)
	}

	// Past TTL: not usable, and store does not leak the entry.
	store.put("s2", "v2")
	now = now.Add(loginSessionTTL + time.Second)
	if got, ok := store.take("s2"); ok {
		t.Fatalf("past TTL: take = %q, %v; want \"\", false", got, ok)
	}
	if _, present := store.sessions["s2"]; present {
		t.Fatalf("expired entry was not removed from store")
	}
}

func TestPKCEChallengeIsS256(t *testing.T) {
	verifier := oauth2.GenerateVerifier()
	cfg := &oauth2.Config{
		ClientID: "kubernetes",
		Endpoint: oauth2.Endpoint{AuthURL: "https://idp.example/authorize"},
	}
	authURL, err := url.Parse(cfg.AuthCodeURL("state-x", oauth2.S256ChallengeOption(verifier)))
	if err != nil {
		t.Fatalf("parse authCodeURL: %v", err)
	}
	q := authURL.Query()
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method = %q; want S256", got)
	}
	challenge := q.Get("code_challenge")
	if challenge == "" {
		t.Fatalf("code_challenge missing from authorize URL")
	}
	if challenge == verifier {
		t.Fatalf("code_challenge equals verifier; expected SHA-256 digest")
	}
	if got := oauth2.S256ChallengeFromVerifier(verifier); got != challenge {
		t.Fatalf("code_challenge %q does not match S256ChallengeFromVerifier %q", challenge, got)
	}
}
