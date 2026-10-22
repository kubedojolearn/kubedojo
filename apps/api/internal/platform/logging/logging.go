// Package logging builds the structured JSON logger used by every binary.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// Redacted replaces the value of any attribute whose key looks sensitive.
const Redacted = "[REDACTED]"

var sensitiveKeyParts = []string{"authorization", "cookie", "password", "secret", "token", "ticket", "assertion", "private_key", "database_url", "redis_url"}

// IsSensitiveKey reports whether a log attribute key must never carry its real value.
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, part := range sensitiveKeyParts {
		if strings.Contains(k, part) {
			return true
		}
	}
	return false
}

// New returns a JSON logger at the given level ("debug", "info", "warn", "error") that redacts
// sensitive attributes by key, including inside groups.
func New(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if IsSensitiveKey(a.Key) {
				return slog.String(a.Key, Redacted)
			}
			return a
		},
	}))
}
