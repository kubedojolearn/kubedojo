package auth

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Assertion constants (ADR-0003).
const (
	Issuer             = "kubedojo-web"
	AudienceAPI        = "kubedojo-api"
	AudienceProvision  = "kubedojo-api-provision"
	MaxLifetime        = 60 * time.Second
	ClockSkew          = 10 * time.Second
	maxTokenBytes      = 4096
	ProviderGitHub     = "github"
	maxProfileClaimLen = 512
)

var (
	// sub is github:<numeric account id>; never a username or email.
	subjectPattern = regexp.MustCompile(`^github:([1-9][0-9]{0,19})$`)
	jtiPattern     = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)
)

// ErrInvalidAssertion is returned for every verification failure. Callers must not reveal
// which check failed; the reason is only for logs.
var ErrInvalidAssertion = errors.New("invalid assertion")

// Claims are the verified contents of an assertion.
type Claims struct {
	Provider        string
	ProviderSubject string
	JTI             string
	KeyID           string
	IssuedAt        time.Time
	ExpiresAt       time.Time
	// Profile claims; present only on provisioning assertions.
	Login     string
	Name      string
	AvatarURL string
}

type rawClaims struct {
	jwt.RegisteredClaims
	Login     string `json:"login,omitempty"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Verifier checks assertions against the configured public keys.
type Verifier struct {
	keys map[string]ed25519.PublicKey
	now  func() time.Time
}

// NewVerifier returns a Verifier. now is injectable for tests.
func NewVerifier(keys map[string]ed25519.PublicKey, now func() time.Time) *Verifier {
	if now == nil {
		now = time.Now
	}
	return &Verifier{keys: keys, now: now}
}

// Verify checks signature, algorithm, key ID, issuer, the exact audience, timing and subject.
// Only EdDSA is accepted, so "none", HS256-with-public-key and RS/ES confusion attacks fail.
func (v *Verifier) Verify(token, audience string) (Claims, error) {
	fail := func(reason string) (Claims, error) {
		return Claims{}, fmt.Errorf("%w: %s", ErrInvalidAssertion, reason)
	}
	if audience != AudienceAPI && audience != AudienceProvision {
		return fail("unknown audience requested")
	}
	if len(token) == 0 || len(token) > maxTokenBytes {
		return fail("missing or oversized token")
	}

	var kid string
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(ClockSkew),
		jwt.WithTimeFunc(v.now),
	)
	var raw rawClaims
	_, err := parser.ParseWithClaims(token, &raw, func(t *jwt.Token) (any, error) {
		k, ok := t.Header["kid"].(string)
		if !ok || k == "" {
			return nil, errors.New("missing kid")
		}
		key, ok := v.keys[k]
		if !ok {
			return nil, errors.New("unknown kid")
		}
		kid = k
		return key, nil
	})
	if err != nil {
		return fail(err.Error())
	}

	// The audience must be exactly the one expected: a token valid for both API and
	// provisioning would let one assertion be replayed against the other endpoint.
	if len(raw.Audience) != 1 {
		return fail("audience must have exactly one value")
	}
	if raw.IssuedAt == nil || raw.ExpiresAt == nil {
		return fail("iat and exp are required")
	}
	iat, exp := raw.IssuedAt.Time, raw.ExpiresAt.Time
	if !exp.After(iat) || exp.Sub(iat) > MaxLifetime {
		return fail("lifetime must be positive and at most 60s")
	}
	if raw.NotBefore != nil && raw.NotBefore.After(v.now().Add(ClockSkew)) {
		return fail("not yet valid")
	}
	m := subjectPattern.FindStringSubmatch(raw.Subject)
	if m == nil {
		return fail("subject must be github:<numeric id>")
	}
	if !jtiPattern.MatchString(raw.ID) {
		return fail("jti is required")
	}

	c := Claims{
		Provider: ProviderGitHub, ProviderSubject: m[1], JTI: raw.ID, KeyID: kid,
		IssuedAt: iat, ExpiresAt: exp,
	}
	if audience == AudienceProvision {
		if raw.Login == "" || len(raw.Login) > 39 || len(raw.Name) > maxProfileClaimLen || len(raw.AvatarURL) > maxProfileClaimLen {
			return fail("provisioning profile claims missing or too long")
		}
		c.Login, c.Name, c.AvatarURL = raw.Login, raw.Name, raw.AvatarURL
	} else if raw.Login != "" || raw.Name != "" || raw.AvatarURL != "" {
		// Request assertions carry identity only (ADR-0003).
		return fail("request assertions must not carry profile claims")
	}
	return c, nil
}
