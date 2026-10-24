// Package authtest mints assertions for tests, including deliberately malformed ones.
// It must only be imported from _test.go files.
package authtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Signer holds a test Ed25519 key pair.
type Signer struct {
	KID     string
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

// NewSigner generates a key pair with the given kid.
func NewSigner(t testing.TB, kid string) Signer {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return Signer{KID: kid, Private: priv, Public: pub}
}

// Keys returns a key set containing this signer's public key.
func (s Signer) Keys() map[string]ed25519.PublicKey {
	return map[string]ed25519.PublicKey{s.KID: s.Public}
}

// Claims returns valid request-assertion claims for GitHub account id at time now.
func Claims(githubID string, now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": "kubedojo-web",
		"aud": "kubedojo-api",
		"sub": "github:" + githubID,
		"iat": now.Unix(),
		"exp": now.Add(60 * time.Second).Unix(),
		"jti": uuid.NewString(),
	}
}

// Sign signs claims with EdDSA and this signer's kid.
func (s Signer) Sign(t testing.TB, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = s.KID
	out, err := tok.SignedString(s.Private)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Token is shorthand for a valid request assertion.
func (s Signer) Token(t testing.TB, githubID string, now time.Time) string {
	t.Helper()
	return s.Sign(t, Claims(githubID, now))
}
