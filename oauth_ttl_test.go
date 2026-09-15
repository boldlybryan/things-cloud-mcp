package main

import (
	"testing"
	"time"
)

func TestParseAccessTokenTTLDefaultIs30Days(t *testing.T) {
	got := parseAccessTokenTTL("")
	if got != 30*24*time.Hour {
		t.Fatalf("default TTL = %v, want 30 days", got)
	}
	if secs := int64(got.Seconds()); secs != defaultAccessTokenTTLSecs {
		t.Fatalf("default TTL seconds = %d, want %d", secs, defaultAccessTokenTTLSecs)
	}
}

func TestParseAccessTokenTTLFromSeconds(t *testing.T) {
	if got := parseAccessTokenTTL("7200"); got != 2*time.Hour {
		t.Fatalf("got %v, want 2h", got)
	}
	if got := parseAccessTokenTTL(" 90 "); got != 90*time.Second {
		t.Fatalf("got %v, want 90s", got)
	}
}

func TestParseAccessTokenTTLRejectsNonPositive(t *testing.T) {
	for _, raw := range []string{"0", "-1", "bogus", "1.5"} {
		if got := parseAccessTokenTTL(raw); got != 30*24*time.Hour {
			t.Fatalf("%q => %v, want default 30 days", raw, got)
		}
	}
}

func TestNewOAuthServerUsesConfiguredAccessTokenTTL(t *testing.T) {
	t.Setenv(accessTokenTTLEnv, "120")
	t.Setenv("CREDENTIALS_SECRET", "")
	o := NewOAuthServer(NewUserManager(), t.TempDir())
	defer o.db.Close()

	now := time.Now()
	token, expiresIn, err := o.newAccessToken("user@example.com", "https://example.test")
	if err != nil {
		t.Fatalf("newAccessToken: %v", err)
	}
	if expiresIn != 120 {
		t.Fatalf("expires_in = %d, want 120", expiresIn)
	}
	claims, err := o.parseJWT(token)
	if err != nil {
		t.Fatalf("parseJWT: %v", err)
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("JWT missing exp")
	}
	wantExp := now.Add(120 * time.Second).Unix()
	if delta := int64(exp) - wantExp; delta < -1 || delta > 1 {
		t.Fatalf("JWT exp = %d, want ~%d (delta %d)", int64(exp), wantExp, delta)
	}
}

func TestNewOAuthServerDefaultAccessTokenTTL(t *testing.T) {
	t.Setenv(accessTokenTTLEnv, "")
	t.Setenv("CREDENTIALS_SECRET", "")
	o := NewOAuthServer(NewUserManager(), t.TempDir())
	defer o.db.Close()

	now := time.Now()
	token, expiresIn, err := o.newAccessToken("user@example.com", "https://example.test")
	if err != nil {
		t.Fatalf("newAccessToken: %v", err)
	}
	if expiresIn != defaultAccessTokenTTLSecs {
		t.Fatalf("expires_in = %d, want %d", expiresIn, defaultAccessTokenTTLSecs)
	}
	claims, err := o.parseJWT(token)
	if err != nil {
		t.Fatalf("parseJWT: %v", err)
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("JWT missing exp")
	}
	wantExp := now.Add(time.Duration(defaultAccessTokenTTLSecs) * time.Second).Unix()
	if delta := int64(exp) - wantExp; delta < -1 || delta > 1 {
		t.Fatalf("JWT exp = %d, want ~%d (delta %d)", int64(exp), wantExp, delta)
	}
}

func TestAccessTokenLifetimeNeverZero(t *testing.T) {
	o := &OAuthServer{accessTokenTTL: 0}
	if got := o.accessTokenLifetime(); got <= 0 {
		t.Fatalf("zero-config lifetime = %v, want a finite default", got)
	}
	o.accessTokenTTL = -time.Hour
	if got := o.accessTokenLifetime(); got <= 0 {
		t.Fatalf("negative-config lifetime = %v, want a finite default", got)
	}
}
