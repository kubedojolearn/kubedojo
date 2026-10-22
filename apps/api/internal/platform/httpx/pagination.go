package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
)

// Pagination limits (docs/api/openapi.yaml: limit 1–100, default 20).
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

// Cursors are opaque to clients: base64url(JSON position) + "." + HMAC. The signature stops
// clients from crafting positions; ownership is still enforced by every query.
type Cursors struct {
	key []byte
}

// NewCursors returns a codec signing with key (32+ random bytes, per process is fine:
// a restart only invalidates cursors in flight, which clients handle by restarting the list).
func NewCursors(key []byte) *Cursors { return &Cursors{key: key} }

// Encode returns an opaque cursor for position.
func (c *Cursors) Encode(position any) (string, error) {
	raw, err := json.Marshal(position)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + c.sign(payload), nil
}

// Decode verifies cursor and unmarshals its position into dst.
func (c *Cursors) Decode(cursor string, dst any) error {
	invalid := Validation("Invalid cursor.", FieldError{Field: "cursor", Message: "is invalid or expired"})
	if len(cursor) > 512 {
		return invalid
	}
	dot := -1
	for i := len(cursor) - 1; i >= 0; i-- {
		if cursor[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 {
		return invalid
	}
	payload, sig := cursor[:dot], cursor[dot+1:]
	if !hmac.Equal([]byte(sig), []byte(c.sign(payload))) {
		return invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || json.Unmarshal(raw, dst) != nil {
		return invalid
	}
	return nil
}

func (c *Cursors) sign(payload string) string {
	m := hmac.New(sha256.New, c.key)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}

// PageParams reads `cursor` and `limit` query parameters.
type PageParams struct {
	Cursor string
	Limit  int
}

// ParsePage validates pagination query parameters.
func ParsePage(r *http.Request) (PageParams, error) {
	p := PageParams{Cursor: r.URL.Query().Get("cursor"), Limit: DefaultPageLimit}
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > MaxPageLimit {
			return p, Validation("Invalid limit.", FieldError{Field: "limit", Message: "must be an integer from 1 to 100"})
		}
		p.Limit = n
	}
	return p, nil
}
