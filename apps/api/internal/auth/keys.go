// Package auth verifies the BFF's signed assertions (ADR-0003) and turns them into a
// request-scoped Principal whose user ID and role come from the database.
package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

var kidPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// ParsePublicKeys parses API_JWT_PUBLIC_KEYS: comma-separated "kid:base64(SPKI DER)" entries.
// Every key must be Ed25519 and every kid unique.
func ParsePublicKeys(s string) (map[string]ed25519.PublicKey, error) {
	keys := map[string]ed25519.PublicKey{}
	for i, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kid, b64, ok := strings.Cut(entry, ":")
		if !ok || !kidPattern.MatchString(kid) {
			return nil, fmt.Errorf("API_JWT_PUBLIC_KEYS entry %d: want kid:base64, with kid of 1-32 [A-Za-z0-9._-]", i+1)
		}
		der, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("API_JWT_PUBLIC_KEYS entry %q: invalid base64", kid)
		}
		pub, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			return nil, fmt.Errorf("API_JWT_PUBLIC_KEYS entry %q: invalid SPKI public key", kid)
		}
		edKey, ok := pub.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("API_JWT_PUBLIC_KEYS entry %q: key is not Ed25519", kid)
		}
		if _, dup := keys[kid]; dup {
			return nil, fmt.Errorf("API_JWT_PUBLIC_KEYS: duplicate kid %q", kid)
		}
		keys[kid] = edKey
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("API_JWT_PUBLIC_KEYS must contain at least one key")
	}
	return keys, nil
}
