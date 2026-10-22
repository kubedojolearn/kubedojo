package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

func spki(t *testing.T, pub any) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func TestParsePublicKeys(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(rand.Reader)
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	keys, err := ParsePublicKeys(" dev-1:" + spki(t, pub1) + " , dev-2:" + spki(t, pub2) + ",")
	if err != nil {
		t.Fatalf("valid keys: %v", err)
	}
	if len(keys) != 2 || !keys["dev-1"].Equal(pub1) || !keys["dev-2"].Equal(pub2) {
		t.Fatalf("unexpected keys %v", keys)
	}

	cases := map[string]string{
		"empty":         "",
		"no kid":        spki(t, pub1),
		"bad kid":       "bad kid:" + spki(t, pub1),
		"bad base64":    "dev-1:%%%",
		"not spki":      "dev-1:" + base64.StdEncoding.EncodeToString([]byte("nope")),
		"rsa key":       "rsa-1:" + spki(t, &rsaKey.PublicKey),
		"duplicate kid": "dev-1:" + spki(t, pub1) + ",dev-1:" + spki(t, pub2),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePublicKeys(in); err == nil {
				t.Fatal("expected error")
			} else if strings.Contains(err.Error(), spki(t, pub1)) {
				t.Fatal("error echoes key material")
			}
		})
	}
}
