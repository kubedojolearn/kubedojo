package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth/authtest"
)

var now = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func fixedNow() time.Time { return now }

func TestVerifyAcceptsValidAssertion(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	v := NewVerifier(s.Keys(), fixedNow)
	c, err := v.Verify(s.Token(t, "583231", now), AudienceAPI)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if c.Provider != "github" || c.ProviderSubject != "583231" || c.KeyID != "dev-1" || c.JTI == "" {
		t.Fatalf("claims %+v", c)
	}
}

func TestVerifyRejects(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	other := authtest.NewSigner(t, "dev-1") // same kid, different key
	v := NewVerifier(s.Keys(), fixedNow)

	with := func(mut func(jwt.MapClaims)) string {
		c := authtest.Claims("583231", now)
		mut(c)
		return s.Sign(t, c)
	}
	valid := s.Token(t, "583231", now)
	parts := strings.Split(valid, ".")

	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	rs256 := jwt.NewWithClaims(jwt.SigningMethodRS256, authtest.Claims("583231", now))
	rs256.Header["kid"] = "dev-1"
	rsToken, _ := rs256.SignedString(rsaKey)

	hs256 := jwt.NewWithClaims(jwt.SigningMethodHS256, authtest.Claims("583231", now))
	hs256.Header["kid"] = "dev-1"
	hsToken, _ := hs256.SignedString([]byte(s.Public)) // classic key-confusion attempt

	noneTok := jwt.NewWithClaims(jwt.SigningMethodNone, authtest.Claims("583231", now))
	noneTok.Header["kid"] = "dev-1"
	noneToken, _ := noneTok.SignedString(jwt.UnsafeAllowNoneSignatureType)

	noKid := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authtest.Claims("583231", now))
	noKidToken, _ := noKid.SignedString(s.Private)

	unknownKid := authtest.Signer{KID: "dev-9", Private: s.Private, Public: s.Public}

	cases := map[string]struct {
		token    string
		audience string
	}{
		"empty":                  {"", AudienceAPI},
		"garbage":                {"not.a.jwt", AudienceAPI},
		"oversized":              {strings.Repeat("a", 5000), AudienceAPI},
		"tampered payload":       {parts[0] + "." + strings.Split(with(func(c jwt.MapClaims) { c["sub"] = "github:1" }), ".")[1] + "." + parts[2], AudienceAPI},
		"signed by another key":  {other.Token(t, "583231", now), AudienceAPI},
		"alg none":               {noneToken, AudienceAPI},
		"HS256 with public key":  {hsToken, AudienceAPI},
		"RS256":                  {rsToken, AudienceAPI},
		"missing kid":            {noKidToken, AudienceAPI},
		"unknown kid":            {unknownKid.Token(t, "583231", now), AudienceAPI},
		"wrong issuer":           {with(func(c jwt.MapClaims) { c["iss"] = "evil" }), AudienceAPI},
		"wrong audience":         {with(func(c jwt.MapClaims) { c["aud"] = "something-else" }), AudienceAPI},
		"provision token on API": {with(func(c jwt.MapClaims) { c["aud"] = AudienceProvision; c["login"] = "octo" }), AudienceAPI},
		"API token on provision": {valid, AudienceProvision},
		"two audiences":          {with(func(c jwt.MapClaims) { c["aud"] = []string{AudienceAPI, AudienceProvision} }), AudienceAPI},
		"expired": {with(func(c jwt.MapClaims) {
			c["iat"] = now.Add(-2 * time.Minute).Unix()
			c["exp"] = now.Add(-61 * time.Second).Unix()
		}), AudienceAPI},
		"issued in the future": {with(func(c jwt.MapClaims) {
			c["iat"] = now.Add(time.Minute).Unix()
			c["exp"] = now.Add(90 * time.Second).Unix()
		}), AudienceAPI},
		"lifetime over 60s":           {with(func(c jwt.MapClaims) { c["exp"] = now.Add(10 * time.Minute).Unix() }), AudienceAPI},
		"exp before iat":              {with(func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Second).Unix() }), AudienceAPI},
		"missing exp":                 {with(func(c jwt.MapClaims) { delete(c, "exp") }), AudienceAPI},
		"missing iat":                 {with(func(c jwt.MapClaims) { delete(c, "iat") }), AudienceAPI},
		"not yet valid":               {with(func(c jwt.MapClaims) { c["nbf"] = now.Add(time.Minute).Unix() }), AudienceAPI},
		"missing subject":             {with(func(c jwt.MapClaims) { delete(c, "sub") }), AudienceAPI},
		"username as subject":         {with(func(c jwt.MapClaims) { c["sub"] = "github:octocat" }), AudienceAPI},
		"leading zero subject":        {with(func(c jwt.MapClaims) { c["sub"] = "github:0123" }), AudienceAPI},
		"other provider":              {with(func(c jwt.MapClaims) { c["sub"] = "gitlab:123" }), AudienceAPI},
		"email as subject":            {with(func(c jwt.MapClaims) { c["sub"] = "octo@example.com" }), AudienceAPI},
		"missing jti":                 {with(func(c jwt.MapClaims) { delete(c, "jti") }), AudienceAPI},
		"profile claims on API token": {with(func(c jwt.MapClaims) { c["login"] = "octo" }), AudienceAPI},
		"role claim smuggled":         {with(func(c jwt.MapClaims) { c["role"] = "admin"; c["login"] = "x" }), AudienceAPI},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(tc.token, tc.audience); !errors.Is(err, ErrInvalidAssertion) {
				t.Fatalf("expected ErrInvalidAssertion, got %v", err)
			}
		})
	}
}

func TestVerifyIgnoresUnknownClaims(t *testing.T) {
	// A "role" claim is meaningless: roles come only from the database.
	s := authtest.NewSigner(t, "dev-1")
	c := authtest.Claims("42", now)
	c["role"] = "admin"
	got, err := NewVerifier(s.Keys(), fixedNow).Verify(s.Sign(t, c), AudienceAPI)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.ProviderSubject != "42" {
		t.Fatalf("claims %+v", got)
	}
}

func TestVerifyClockSkew(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	v := NewVerifier(s.Keys(), fixedNow)
	within := authtest.Claims("1", now)
	within["iat"] = now.Add(8 * time.Second).Unix() // web clock slightly ahead
	within["exp"] = now.Add(60 * time.Second).Unix()
	if _, err := v.Verify(s.Sign(t, within), AudienceAPI); err != nil {
		t.Fatalf("8s skew should be accepted: %v", err)
	}
	expiredWithinSkew := authtest.Claims("1", now.Add(-65*time.Second))
	if _, err := v.Verify(s.Sign(t, expiredWithinSkew), AudienceAPI); err != nil {
		t.Fatalf("expired 5s ago is within the 10s leeway: %v", err)
	}
	expired := authtest.Claims("1", now.Add(-75*time.Second))
	if _, err := v.Verify(s.Sign(t, expired), AudienceAPI); err == nil {
		t.Fatal("expired 15s ago must be rejected")
	}
}

func TestReplayWindowIsBoundedByLifetime(t *testing.T) {
	// ADR-0003 accepts replay within the assertion's lifetime (no jti store); this test pins
	// that decision and proves the window closes at exp + skew.
	s := authtest.NewSigner(t, "dev-1")
	token := s.Token(t, "7", now)
	clock := now
	v := NewVerifier(s.Keys(), func() time.Time { return clock })
	for _, offset := range []time.Duration{0, 30 * time.Second, 69 * time.Second} {
		clock = now.Add(offset)
		if _, err := v.Verify(token, AudienceAPI); err != nil {
			t.Fatalf("at +%v: %v", offset, err)
		}
	}
	clock = now.Add(71 * time.Second)
	if _, err := v.Verify(token, AudienceAPI); err == nil {
		t.Fatal("replay after exp+skew must fail")
	}
}

func TestProvisioningAssertion(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	v := NewVerifier(s.Keys(), fixedNow)
	c := authtest.Claims("583231", now)
	c["aud"] = AudienceProvision
	c["login"] = "octocat"
	c["name"] = "The Octocat"
	c["avatar_url"] = "https://avatars.githubusercontent.com/u/583231"
	got, err := v.Verify(s.Sign(t, c), AudienceProvision)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Login != "octocat" || got.Name != "The Octocat" || got.AvatarURL == "" {
		t.Fatalf("claims %+v", got)
	}

	delete(c, "login")
	if _, err := v.Verify(s.Sign(t, c), AudienceProvision); err == nil {
		t.Fatal("provisioning without login must fail")
	}
	c["login"] = strings.Repeat("a", 40)
	if _, err := v.Verify(s.Sign(t, c), AudienceProvision); err == nil {
		t.Fatal("login over 39 chars must fail")
	}
}

func TestKeyRotation(t *testing.T) {
	oldKey := authtest.NewSigner(t, "prod-1")
	newKey := authtest.NewSigner(t, "prod-2")
	keys := oldKey.Keys()
	keys[newKey.KID] = newKey.Public
	v := NewVerifier(keys, fixedNow)
	for _, s := range []authtest.Signer{oldKey, newKey} {
		if _, err := v.Verify(s.Token(t, "1", now), AudienceAPI); err != nil {
			t.Fatalf("%s: %v", s.KID, err)
		}
	}
	afterRotation := NewVerifier(newKey.Keys(), fixedNow)
	if _, err := afterRotation.Verify(oldKey.Token(t, "1", now), AudienceAPI); err == nil {
		t.Fatal("removed key must stop working")
	}
}

func TestVerifyRejectsUnknownAudienceRequest(t *testing.T) {
	s := authtest.NewSigner(t, "dev-1")
	if _, err := NewVerifier(s.Keys(), fixedNow).Verify(s.Token(t, "1", now), "anything"); err == nil {
		t.Fatal("expected error")
	}
}
