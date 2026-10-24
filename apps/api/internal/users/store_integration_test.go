//go:build integration

package users_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

var ctx = context.Background()

func input(subject, login string) users.ProvisionInput {
	return users.ProvisionInput{
		Provider: "github", ProviderSubject: subject, Login: login, Name: "Name of " + login,
		AvatarURL: "https://avatars.githubusercontent.com/u/" + subject,
	}
}

func TestProvisionCreatesOnceAndReuses(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)

	first, err := s.Provision(ctx, input("583231", "octocat"))
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	again, err := s.Provision(ctx, input("583231", "octocat-renamed"))
	if err != nil || again.Created || again.UserID != first.UserID {
		t.Fatalf("repeat sign-in must reuse the user: %+v %v", again, err)
	}

	me, err := s.GetMe(ctx, first.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if me.Handle != "octocat" {
		t.Fatalf("handle must not follow GitHub renames, got %q", me.Handle)
	}
	if me.Role != auth.RoleLearner || me.Settings.Theme != "system" || me.Settings.ProfilePublic || me.Settings.LeaderboardOptIn {
		t.Fatalf("defaults: %+v", me)
	}
	var totals int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM user_totals WHERE user_id = $1", first.UserID).Scan(&totals)
	if totals != 1 {
		t.Fatal("user_totals row missing")
	}
}

func TestDistinctIdentitiesNeverMerge(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	a, err := s.Provision(ctx, input("100", "same-login"))
	if err != nil {
		t.Fatal(err)
	}
	// Same login and display name, different immutable account ID: a different person.
	b, err := s.Provision(ctx, input("200", "same-login"))
	if err != nil {
		t.Fatal(err)
	}
	if a.UserID == b.UserID || !b.Created {
		t.Fatal("identities merged by login")
	}
	meB, _ := s.GetMe(ctx, b.UserID)
	if meB.Handle != "same-login-2" {
		t.Fatalf("collision suffix: %q", meB.Handle)
	}
}

func TestHandleCollisionsAreCaseInsensitive(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	if _, err := s.Provision(ctx, input("1", "KubeFan")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Provision(ctx, input("2", "kubefan"))
	if err != nil {
		t.Fatal(err)
	}
	me, _ := s.GetMe(ctx, r.UserID)
	if !strings.EqualFold(me.Handle, "kubefan-2") {
		t.Fatalf("got %q", me.Handle)
	}
}

func TestHandleSuffixFitsLengthLimit(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	long := strings.Repeat("a", 39)
	for i := 1; i <= 11; i++ {
		r, err := s.Provision(ctx, input("1"+strings.Repeat("0", i), long))
		if err != nil {
			t.Fatalf("provision %d: %v", i, err)
		}
		me, _ := s.GetMe(ctx, r.UserID)
		if len(me.Handle) > 39 {
			t.Fatalf("handle too long: %q", me.Handle)
		}
	}
}

func TestConcurrentFirstSignInConverges(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	const n = 10
	ids := make([]uuid.UUID, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Go(func() {
			r, err := s.Provision(ctx, input("4242", "racer"))
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = r.UserID
		})
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatal("concurrent sign-ins created more than one user")
		}
	}
	var count int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count)
	if count != 1 {
		t.Fatalf("%d users", count)
	}
}

func TestConcurrentHandleRace(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	var wg sync.WaitGroup
	for i := 1; i <= 8; i++ {
		wg.Go(func() {
			if _, err := s.Provision(ctx, input(strings.Repeat("9", i), "popular")); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var distinct int
	_ = pool.QueryRow(ctx, "SELECT count(DISTINCT lower(handle)) FROM users").Scan(&distinct)
	if distinct != 8 {
		t.Fatalf("expected 8 distinct handles, got %d", distinct)
	}
}

func TestDisplayNamePreferenceSurvivesSignIn(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	r, _ := s.Provision(ctx, input("7", "dev"))
	if _, err := pool.Exec(ctx, "UPDATE users SET display_name = 'Chosen Name', display_name_customized = true WHERE id = $1", r.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Provision(ctx, input("7", "dev")); err != nil {
		t.Fatal(err)
	}
	me, _ := s.GetMe(ctx, r.UserID)
	if me.DisplayName != "Chosen Name" {
		t.Fatalf("customized display name overwritten: %q", me.DisplayName)
	}
}

func TestProfileFieldsAreSanitized(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	in := input("9", "x")
	in.Name = "  Ada ‮\u0000 Lovelace  \n" + strings.Repeat("y", 200)
	in.AvatarURL = "javascript:alert(1)"
	r, err := s.Provision(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	me, _ := s.GetMe(ctx, r.UserID)
	if !strings.HasPrefix(me.DisplayName, "Ada Lovelace yyy") || len([]rune(me.DisplayName)) != 80 {
		t.Fatalf("display name %q", me.DisplayName)
	}
	if me.AvatarURL != nil {
		t.Fatalf("unsafe avatar kept: %v", *me.AvatarURL)
	}
}

func TestResolve(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	r, _ := s.Provision(ctx, input("31", "resolver"))

	p, err := s.Resolve(ctx, "github", "31")
	if err != nil || p.UserID != r.UserID || p.Role != auth.RoleLearner {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Resolve(ctx, "github", "32"); !errors.Is(err, auth.ErrIdentityNotLinked) {
		t.Fatalf("unknown identity: %v", err)
	}

	// Role changes in the database take effect on the next request.
	if _, err := pool.Exec(ctx, "UPDATE users SET role = 'admin' WHERE id = $1", r.UserID); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Resolve(ctx, "github", "31"); p.Role != auth.RoleAdmin {
		t.Fatal("role must come from the database")
	}

	// A tombstoned user no longer resolves, even before the identity is unlinked.
	if _, err := pool.Exec(ctx, "UPDATE users SET deleted_at = now() WHERE id = $1", r.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, "github", "31"); !errors.Is(err, auth.ErrIdentityNotLinked) {
		t.Fatalf("deleted user resolved: %v", err)
	}
	if _, err := s.GetMe(ctx, r.UserID); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("deleted user readable: %v", err)
	}
}

func TestSignInAfterAccountDeletionCreatesFreshAccount(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	old, _ := s.Provision(ctx, input("55", "returner"))
	// Account deletion: tombstone and unlink the identity (ADR-0003).
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "UPDATE users SET deleted_at = now() WHERE id = $1", old.UserID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "DELETE FROM oauth_identities WHERE user_id = $1", old.UserID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := s.Provision(ctx, input("55", "returner"))
	if err != nil || !fresh.Created || fresh.UserID == old.UserID {
		t.Fatalf("expected a new account: %+v %v", fresh, err)
	}
	me, _ := s.GetMe(ctx, fresh.UserID)
	if me.Handle == "returner" {
		t.Fatal("the deleted user's handle must stay reserved")
	}
}

func ptr[T any](v T) *T { return &v }

func TestUpdateMe(t *testing.T) {
	pool := testdb.New(t)
	s := users.NewStore(pool)
	a, _ := s.Provision(ctx, input("1", "alice"))
	_, _ = s.Provision(ctx, input("2", "bob"))

	req := users.UpdateMeRequest{
		DisplayName:        ptr("Alice L."),
		Handle:             ptr("Alice-Learns"),
		Settings:           &users.SettingsPatch{Theme: ptr("dark"), TimeZone: ptr("Asia/Colombo"), LeaderboardOptIn: ptr(true)},
		Onboarding:         &users.OnboardingInfo{ExperienceLevel: ptr("some")},
		CompleteOnboarding: ptr(true),
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateMe(ctx, a.UserID, req); err != nil {
		t.Fatal(err)
	}
	me, _ := s.GetMe(ctx, a.UserID)
	if me.DisplayName != "Alice L." || me.Handle != "Alice-Learns" || me.Settings.Theme != "dark" ||
		me.Settings.TimeZone != "Asia/Colombo" || !me.Settings.LeaderboardOptIn || me.Settings.ProfilePublic || me.OnboardingCompletedAt == nil {
		t.Fatalf("after update: %+v", me)
	}
	firstCompleted := *me.OnboardingCompletedAt

	// Completing onboarding again keeps the original timestamp; partial updates touch nothing else.
	if err := s.UpdateMe(ctx, a.UserID, users.UpdateMeRequest{CompleteOnboarding: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	me, _ = s.GetMe(ctx, a.UserID)
	if !me.OnboardingCompletedAt.Equal(firstCompleted) || me.Settings.Theme != "dark" {
		t.Fatalf("partial update changed other fields: %+v", me)
	}

	// The chosen display name survives the next GitHub sign-in.
	if _, err := s.Provision(ctx, input("1", "alice")); err != nil {
		t.Fatal(err)
	}
	if me, _ = s.GetMe(ctx, a.UserID); me.DisplayName != "Alice L." {
		t.Fatalf("display name overwritten by sign-in: %q", me.DisplayName)
	}

	if err := s.UpdateMe(ctx, a.UserID, users.UpdateMeRequest{Handle: ptr("BOB")}); !errors.Is(err, users.ErrHandleTaken) {
		t.Fatalf("case-insensitive handle conflict: %v", err)
	}
}
