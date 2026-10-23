package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// UpdateMeRequest mirrors the OpenAPI UpdateMeRequest. Only these fields can be changed by the
// user; role, XP, IDs and timestamps are not representable (unknown fields are rejected).
type UpdateMeRequest struct {
	DisplayName        *string         `json:"displayName"`
	Handle             *string         `json:"handle"`
	Settings           *SettingsPatch  `json:"settings"`
	Onboarding         *OnboardingInfo `json:"onboarding"`
	CompleteOnboarding *bool           `json:"completeOnboarding"`
}

// SettingsPatch is a partial Settings update.
type SettingsPatch struct {
	Theme            *string `json:"theme"`
	ReducedMotion    *bool   `json:"reducedMotion"`
	LeaderboardOptIn *bool   `json:"leaderboardOptIn"`
	ProfilePublic    *bool   `json:"profilePublic"`
	AnalyticsOptOut  *bool   `json:"analyticsOptOut"`
	TimeZone         *string `json:"timeZone"`
}

// OnboardingInfo holds the optional onboarding answers.
type OnboardingInfo struct {
	ExperienceLevel *string `json:"experienceLevel"`
	LearningGoal    *string `json:"learningGoal"`
}

// Handles that would be confusing or impersonate the project in profile URLs.
var reservedHandles = map[string]bool{
	"admin": true, "administrator": true, "api": true, "auth": true, "dashboard": true, "help": true,
	"kubedojo": true, "kubernetes": true, "learn": true, "login": true, "logout": true, "me": true,
	"moderator": true, "root": true, "settings": true, "support": true, "system": true,
}

var (
	themes           = map[string]bool{"system": true, "dark": true, "light": true}
	experienceLevels = map[string]bool{"new": true, "some": true, "comfortable": true}
	learningGoals    = map[string]bool{"fundamentals": true, "troubleshooting": true, "certification-prep": true, "work-project": true, "curiosity": true}
)

// Validate normalizes and checks the request, returning field-level errors.
func (r *UpdateMeRequest) Validate() error {
	var details []httpx.FieldError
	add := func(field, msg string) { details = append(details, httpx.FieldError{Field: field, Message: msg}) }

	if r.DisplayName == nil && r.Handle == nil && r.Settings == nil && r.Onboarding == nil && r.CompleteOnboarding == nil {
		return httpx.Validation("Nothing to update.")
	}
	if r.DisplayName != nil {
		clean := sanitizeDisplayName(*r.DisplayName, "")
		if clean == "" {
			add("displayName", "must contain at least one visible character")
		}
		r.DisplayName = &clean
	}
	if r.Handle != nil {
		h := strings.TrimSpace(*r.Handle)
		switch {
		case !handlePattern.MatchString(h):
			add("handle", "must be 1-39 letters, numbers or single hyphens, not starting or ending with a hyphen")
		case strings.Contains(h, "--"):
			add("handle", "must not contain consecutive hyphens")
		case reservedHandles[strings.ToLower(h)]:
			add("handle", "is reserved")
		}
		r.Handle = &h
	}
	if s := r.Settings; s != nil {
		if s.Theme != nil && !themes[*s.Theme] {
			add("settings.theme", "must be system, dark or light")
		}
		if s.TimeZone != nil {
			if len(*s.TimeZone) > 64 || *s.TimeZone == "" || *s.TimeZone == "Local" {
				add("settings.timeZone", "must be an IANA time zone such as Europe/London")
			} else if _, err := time.LoadLocation(*s.TimeZone); err != nil {
				add("settings.timeZone", "must be an IANA time zone such as Europe/London")
			}
		}
	}
	if o := r.Onboarding; o != nil {
		if o.ExperienceLevel != nil && !experienceLevels[*o.ExperienceLevel] {
			add("onboarding.experienceLevel", "is not a valid option")
		}
		if o.LearningGoal != nil && !learningGoals[*o.LearningGoal] {
			add("onboarding.learningGoal", "is not a valid option")
		}
	}
	if r.CompleteOnboarding != nil && !*r.CompleteOnboarding {
		add("completeOnboarding", "can only be true")
	}
	if len(details) > 0 {
		return httpx.Validation("Some fields are invalid.", details...)
	}
	return nil
}

// ErrHandleTaken means another account already uses the handle (case-insensitively).
var ErrHandleTaken = errors.New("handle taken")

// UpdateMe applies a validated update in one transaction.
func (s *Store) UpdateMe(ctx context.Context, userID uuid.UUID, r UpdateMeRequest) error {
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		p := dbgen.UpdateProfileParams{ID: userID, DisplayName: r.DisplayName, Handle: r.Handle}
		if r.Onboarding != nil {
			p.ExperienceLevel, p.LearningGoal = r.Onboarding.ExperienceLevel, r.Onboarding.LearningGoal
		}
		p.CompleteOnboarding = r.CompleteOnboarding != nil && *r.CompleteOnboarding
		if err := q.UpdateProfile(ctx, p); err != nil {
			return err
		}
		if st := r.Settings; st != nil {
			return q.UpdateSettings(ctx, dbgen.UpdateSettingsParams{
				UserID: userID, Theme: st.Theme, ReducedMotion: st.ReducedMotion, LeaderboardOptIn: st.LeaderboardOptIn,
				ProfilePublic: st.ProfilePublic, AnalyticsOptOut: st.AnalyticsOptOut, TimeZone: st.TimeZone,
			})
		}
		return nil
	})
	if database.IsCode(err, database.CodeUniqueViolation) && database.ConstraintName(err) == "users_handle_lower_key" {
		return ErrHandleTaken
	}
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}
