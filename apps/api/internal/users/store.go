package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// Store is the PostgreSQL user store. It implements auth.IdentityResolver and Reader.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var _ auth.IdentityResolver = (*Store)(nil)
var _ Reader = (*Store)(nil)

// Resolve maps a provider identity to its active user and current role.
func (s *Store) Resolve(ctx context.Context, provider, subject string) (auth.Principal, error) {
	row, err := dbgen.New(s.pool).ResolveIdentity(ctx, dbgen.ResolveIdentityParams{Provider: provider, ProviderSubject: subject})
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, auth.ErrIdentityNotLinked
	}
	if err != nil {
		return auth.Principal{}, fmt.Errorf("resolve identity: %w", err)
	}
	return auth.Principal{UserID: row.ID, Role: auth.Role(row.Role), Provider: provider, ProviderSubject: subject}, nil
}

// GetMe loads the user's profile and settings.
func (s *Store) GetMe(ctx context.Context, id uuid.UUID) (Me, error) {
	r, err := dbgen.New(s.pool).GetMe(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Me{}, ErrNotFound
	}
	if err != nil {
		return Me{}, fmt.Errorf("get user: %w", err)
	}
	return Me{
		ID: r.ID, Handle: r.Handle, DisplayName: r.DisplayName, AvatarURL: r.AvatarUrl, Role: auth.Role(r.Role),
		CreatedAt: r.CreatedAt, OnboardingCompletedAt: r.OnboardingCompletedAt,
		Settings: Settings{
			Theme: r.Theme, ReducedMotion: r.ReducedMotion, LeaderboardOptIn: r.LeaderboardOptIn,
			ProfilePublic: r.ProfilePublic, AnalyticsOptOut: r.AnalyticsOptOut, TimeZone: r.TimeZone,
		},
	}, nil
}

// ProvisionInput comes from a verified provisioning assertion (ADR-0003).
type ProvisionInput struct {
	Provider        string
	ProviderSubject string
	Login           string
	Name            string
	AvatarURL       string
}

// ProvisionResult reports the local user for the identity.
type ProvisionResult struct {
	UserID  uuid.UUID
	Created bool
}

const maxProvisionAttempts = 5

// Provision creates the user for a new identity or refreshes the provider-owned profile fields
// of an existing one, atomically. Concurrent first sign-ins for the same identity converge on
// one user; handle collisions get a numeric suffix. Users are never matched by email or login.
func (s *Store) Provision(ctx context.Context, in ProvisionInput) (ProvisionResult, error) {
	name := sanitizeDisplayName(in.Name, in.Login)
	avatar := sanitizeAvatarURL(in.AvatarURL)
	for attempt := 0; attempt < maxProvisionAttempts; attempt++ {
		var res ProvisionResult
		err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			existing, err := q.GetIdentityForUpdate(ctx, dbgen.GetIdentityForUpdateParams{Provider: in.Provider, ProviderSubject: in.ProviderSubject})
			switch {
			case err == nil:
				if existing.DeletedAt != nil {
					// Deletion unlinks identities, so this means an erasure is still in progress.
					return errors.New("identity is linked to a deleted user")
				}
				if err := q.TouchIdentity(ctx, dbgen.TouchIdentityParams{Provider: in.Provider, ProviderSubject: in.ProviderSubject, ProviderLogin: in.Login}); err != nil {
					return err
				}
				res = ProvisionResult{UserID: existing.UserID}
				return q.RefreshProviderProfile(ctx, dbgen.RefreshProviderProfileParams{ID: existing.UserID, DisplayName: name, AvatarUrl: avatar})
			case !errors.Is(err, pgx.ErrNoRows):
				return err
			}

			handle, err := availableHandle(ctx, q, in.Login, attempt)
			if err != nil {
				return err
			}
			id, err := q.InsertUser(ctx, dbgen.InsertUserParams{Handle: handle, DisplayName: name, AvatarUrl: avatar})
			if err != nil {
				return err
			}
			if err := q.InsertIdentity(ctx, dbgen.InsertIdentityParams{Provider: in.Provider, ProviderSubject: in.ProviderSubject, UserID: id, ProviderLogin: in.Login}); err != nil {
				return err
			}
			if err := q.InsertUserSettings(ctx, id); err != nil {
				return err
			}
			if err := q.InsertUserTotals(ctx, id); err != nil {
				return err
			}
			res = ProvisionResult{UserID: id, Created: true}
			return nil
		})
		// A unique violation means a concurrent sign-in won the identity or the handle; retry.
		if database.IsCode(err, database.CodeUniqueViolation) {
			continue
		}
		if err != nil {
			return ProvisionResult{}, fmt.Errorf("provision user: %w", err)
		}
		return res, nil
	}
	return ProvisionResult{}, errors.New("provision user: too many concurrent conflicts")
}

var handlePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

// availableHandle tries login, login-2 … login-9, then login-<random>. After a lost race
// (attempt > 0) it skips straight to random suffixes.
func availableHandle(ctx context.Context, q *dbgen.Queries, login string, attempt int) (string, error) {
	base := login
	if !handlePattern.MatchString(base) {
		base = "learner"
	}
	var candidates []string
	if attempt == 0 {
		candidates = append(candidates, base)
		for n := 2; n <= 9; n++ {
			candidates = append(candidates, withSuffix(base, fmt.Sprintf("%d", n)))
		}
	}
	for i := 0; i < 5; i++ {
		b := make([]byte, 3)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		candidates = append(candidates, withSuffix(base, hex.EncodeToString(b)))
	}
	for _, c := range candidates {
		taken, err := q.HandleTaken(ctx, c)
		if err != nil {
			return "", err
		}
		if !taken {
			return c, nil
		}
	}
	return "", errors.New("no available handle")
}

func withSuffix(base, suffix string) string {
	maxBase := 39 - len(suffix) - 1
	if len(base) > maxBase {
		base = strings.TrimRight(base[:maxBase], "-")
	}
	return base + "-" + suffix
}

// sanitizeDisplayName strips control characters, collapses whitespace and limits length.
func sanitizeDisplayName(name, fallback string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsSpace(r):
			space = true
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			// Drop control and format characters (bidi overrides, zero-width joiners).
		default:
			if space && b.Len() > 0 {
				b.WriteRune(' ')
			}
			space = false
			b.WriteRune(r)
		}
	}
	out := []rune(b.String())
	if len(out) > 80 {
		out = out[:80]
	}
	if len(out) == 0 {
		return fallback
	}
	return string(out)
}

// sanitizeAvatarURL keeps only https URLs on GitHub's avatar host.
func sanitizeAvatarURL(raw string) *string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "avatars.githubusercontent.com" || u.User != nil || len(raw) > 512 {
		return nil
	}
	clean := u.String()
	return &clean
}

// AnalyticsOptOut reports whether the user opted out of learning analytics.
func (s *Store) AnalyticsOptOut(ctx context.Context, id uuid.UUID) (bool, error) {
	return dbgen.New(s.pool).GetAnalyticsOptOut(ctx, id)
}
