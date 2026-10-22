// Package ratelimit implements fixed-window rate limits in Redis and HTTP middleware that
// applies them per user or per client IP (requirements S6).
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// Decision is the outcome of one Allow call.
type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// Limiter counts events per key within a window.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error)
}

// INCR and set the window on first use, atomically; return count and remaining TTL.
var fixedWindow = goredis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
if ttl < 0 then redis.call('PEXPIRE', KEYS[1], ARGV[1]); ttl = tonumber(ARGV[1]) end
return {count, ttl}
`)

// Redis is a Limiter backed by Redis. Keys expire with their window, so nothing accumulates.
type Redis struct {
	client goredis.Scripter
}

// NewRedis returns a Redis limiter.
func NewRedis(client goredis.Scripter) *Redis { return &Redis{client: client} }

// Allow increments the counter for key and reports whether the event is within limit.
func (l *Redis) Allow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error) {
	res, err := fixedWindow.Run(ctx, l.client, []string{"rl:" + key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("rate limit %s: %w", key, err)
	}
	count, ttl := int(res[0]), time.Duration(res[1])*time.Millisecond
	if count > limit {
		return Decision{Allowed: false, RetryAfter: ttl}, nil
	}
	return Decision{Allowed: true, Remaining: limit - count}, nil
}

// Rule is one named limit.
type Rule struct {
	Name   string
	Limit  int
	Window time.Duration
	// FailClosed rejects requests when Redis is unavailable. Use it for abuse-sensitive routes;
	// general limits fail open so a Redis outage doesn't take the API down (ADR-0004).
	FailClosed bool
}

func apply(l Limiter, rule Rule, subject string, w http.ResponseWriter, r *http.Request, next http.Handler) {
	d, err := l.Allow(r.Context(), rule.Name+":"+subject, rule.Limit, rule.Window)
	if err != nil {
		httpx.Logger(r.Context()).Warn("rate limiter unavailable", "rule", rule.Name, "fail_closed", rule.FailClosed, "error", err.Error())
		if rule.FailClosed {
			httpx.WriteError(w, r, &httpx.Error{Status: http.StatusServiceUnavailable, Code: httpx.CodeRateLimited, Message: "Temporarily unavailable. Try again shortly.", RetryAfterSeconds: 5})
			return
		}
		next.ServeHTTP(w, r)
		return
	}
	if !d.Allowed {
		secs := int((d.RetryAfter + time.Second - 1) / time.Second)
		if secs < 1 {
			secs = 1
		}
		httpx.WriteError(w, r, httpx.RateLimited(secs))
		return
	}
	next.ServeHTTP(w, r)
}

// PerUser limits authenticated principals. It must run after auth.Authenticate.
func PerUser(l Limiter, rule Rule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.Unauthenticated())
				return
			}
			apply(l, rule, "user:"+p.UserID.String(), w, r, next)
		})
	}
}

// PerIP limits by client IP as resolved by ips (which honors only trusted proxies).
func PerIP(l Limiter, rule Rule, ips *ClientIPResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apply(l, rule, "ip:"+ips.ClientIP(r), w, r, next)
		})
	}
}
