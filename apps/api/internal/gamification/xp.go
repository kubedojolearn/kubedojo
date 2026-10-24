// Package gamification awards XP from an append-only ledger and maintains derived totals.
package gamification

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
)

// Reasons for XP events (xp_events.reason).
const (
	ReasonLessonCompleted = "lesson_completed"
	ReasonAchievement     = "achievement"
	ReasonAdjustment      = "adjustment"
)

// Grant describes one XP award. EventKey makes it idempotent, e.g. LessonCompletedKey(lessonID).
type Grant struct {
	UserID    uuid.UUID
	EventKey  string
	Delta     int32
	Reason    string
	AttemptID *uuid.UUID
}

// LessonCompletedKey is the event key for completing a lesson; XP for a lesson is awarded once.
func LessonCompletedKey(lessonID uuid.UUID) string { return "lesson-complete:" + lessonID.String() }

// GrantXP records the award and updates the user's total in the caller's transaction. It returns
// granted=false (and changes nothing) if this event key was already awarded, so retries and
// concurrent duplicate requests can never double-count.
func GrantXP(ctx context.Context, q *dbgen.Queries, g Grant) (granted bool, total int32, err error) {
	if g.Delta == 0 {
		return false, 0, errors.New("xp delta must not be zero")
	}
	_, err = q.InsertXPEvent(ctx, dbgen.InsertXPEventParams{
		UserID: g.UserID, EventKey: g.EventKey, Delta: g.Delta, Reason: g.Reason, AttemptID: g.AttemptID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		totals, err := q.GetUserTotals(ctx, g.UserID)
		if err != nil {
			return false, 0, fmt.Errorf("read totals: %w", err)
		}
		return false, totals.TotalXp, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("insert xp event: %w", err)
	}
	total, err = q.AddXP(ctx, dbgen.AddXPParams{UserID: g.UserID, Delta: g.Delta})
	if err != nil {
		return false, 0, fmt.Errorf("update totals: %w", err)
	}
	return true, total, nil
}
