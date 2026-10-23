package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth/authtest"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/health"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

var (
	testUserID = uuid.MustParse("33333333-3333-4333-8333-333333333333")
	throttled  = uuid.MustParse("44444444-4444-4444-8444-444444444444")
)

type identities struct{}

func (identities) Resolve(_ context.Context, _, subject string) (auth.Principal, error) {
	switch subject {
	case "100":
		return auth.Principal{UserID: testUserID, Role: auth.RoleLearner}, nil
	case "429":
		return auth.Principal{UserID: throttled, Role: auth.RoleLearner}, nil
	}
	return auth.Principal{}, auth.ErrIdentityNotLinked
}

type userStore struct{}

func (userStore) GetMe(_ context.Context, id uuid.UUID) (users.Me, error) {
	avatar := "https://avatars.githubusercontent.com/u/100"
	return users.Me{
		ID: id, Handle: "kube-learner", DisplayName: "Kube Learner", AvatarURL: &avatar, Role: auth.RoleLearner,
		CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		Settings:  users.Settings{Theme: "system", TimeZone: "UTC"},
	}, nil
}

func (userStore) UpdateMe(_ context.Context, _ uuid.UUID, r users.UpdateMeRequest) error {
	if r.Handle != nil && *r.Handle == "taken" {
		return users.ErrHandleTaken
	}
	return nil
}

func (s userStore) Provision(_ context.Context, in users.ProvisionInput) (users.ProvisionResult, error) {
	if in.ProviderSubject == "100" {
		return users.ProvisionResult{UserID: testUserID}, nil
	}
	return users.ProvisionResult{UserID: testUserID, Created: true}, nil
}

type limiter struct{}

func (limiter) Allow(ctx context.Context, key string, _ int, _ time.Duration) (ratelimit.Decision, error) {
	if key == "api:user:"+throttled.String() {
		return ratelimit.Decision{Allowed: false, RetryAfter: 30 * time.Second}, nil
	}
	return ratelimit.Decision{Allowed: true}, nil
}

func pinger(err error) health.Pinger {
	return health.PingFunc(func(context.Context) error { return err })
}

// testRouter returns a router wired with in-memory fakes and a signer whose tokens it accepts.
func testRouter(t *testing.T, ready bool) (http.Handler, authtest.Signer) {
	t.Helper()
	s := authtest.NewSigner(t, "test-1")
	var pgErr error
	if !ready {
		pgErr = errors.New("down")
	}
	return NewRouter(Deps{
		Log:            slog.New(slog.DiscardHandler),
		Health:         health.Handler{Postgres: pinger(pgErr), Redis: pinger(nil)},
		Verifier:       auth.NewVerifier(s.Keys(), nil),
		Identities:     identities{},
		Users:          userStore{},
		Limiter:        limiter{},
		RequestTimeout: 5 * time.Second,
		MaxBodyBytes:   128 << 10,
	}), s
}
