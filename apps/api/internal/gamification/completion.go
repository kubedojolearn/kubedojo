package gamification

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
)

// Completion is what passing a lesson's check records.
type Completion struct {
	LessonID  uuid.UUID
	UserID    uuid.UUID
	AttemptID uuid.UUID
	Score     int16
	// XP is the lesson's award for this pass (base + first-try bonus - hint penalty).
	XP int32
	// Activity is false when the learner opted out of activity history.
	Activity bool
	Now      time.Time
}

// Award is an achievement earned by a completion.
type Award struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// Outcome is what a completion changed.
type Outcome struct {
	// FirstCompletion is false if the learner had completed the lesson before (in another lab).
	FirstCompletion bool
	XPAwarded       int32
	Achievements    []Award
}

// CompleteLesson records a passed check in the caller's transaction: the lesson's completion
// and best score, totals and streak (first completion only), the lesson's XP (once per lesson,
// via the ledger's event key) and any achievements this unlocks. Retries and later passes of
// the same lesson award nothing twice.
func CompleteLesson(ctx context.Context, q *dbgen.Queries, c Completion) (Outcome, error) {
	var out Outcome
	n, err := q.CompleteLessonProgress(ctx, dbgen.CompleteLessonProgressParams{UserID: c.UserID, LessonID: c.LessonID, Score: c.Score})
	if err != nil {
		return out, fmt.Errorf("complete lesson: %w", err)
	}
	if n == 0 {
		// Completed before: keep the best score only.
		return out, q.RecordBestScore(ctx, dbgen.RecordBestScoreParams{UserID: c.UserID, LessonID: c.LessonID, Score: c.Score})
	}
	out.FirstCompletion = true

	totals, err := q.LockTotalsWithTimeZone(ctx, c.UserID)
	if err != nil {
		return out, fmt.Errorf("read totals: %w", err)
	}
	today, streak := NextStreak(totals.CurrentStreak, totals.LastCompletionDate, totals.TimeZone, c.Now)
	updated, err := q.RecordCompletionTotals(ctx, dbgen.RecordCompletionTotalsParams{
		UserID: c.UserID, Streak: streak, CompletionDate: &today,
	})
	if err != nil {
		return out, fmt.Errorf("update totals: %w", err)
	}

	if c.XP > 0 {
		attempt := c.AttemptID
		granted, _, err := GrantXP(ctx, q, Grant{
			UserID: c.UserID, EventKey: LessonCompletedKey(c.LessonID), Delta: c.XP, Reason: ReasonLessonCompleted, AttemptID: &attempt,
		})
		if err != nil {
			return out, err
		}
		if granted {
			out.XPAwarded = c.XP
		}
	}
	if c.Activity {
		lesson, xp := c.LessonID, out.XPAwarded
		if err := q.InsertActivity(ctx, dbgen.InsertActivityParams{UserID: c.UserID, Type: "lesson-completed", LessonID: &lesson, XpDelta: &xp}); err != nil {
			return out, err
		}
	}

	out.Achievements, err = awardAchievements(ctx, q, c, updated.CompletedLessons, updated.CurrentStreak)
	return out, err
}

// NextStreak returns today's date in the learner's time zone and the streak after completing a
// lesson today: unchanged if they already completed one today, one longer if their last
// completion was yesterday, otherwise 1.
func NextStreak(current int32, last *time.Time, tz string, now time.Time) (time.Time, int32) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := now.In(loc).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if last == nil {
		return today, 1
	}
	ly, lm, ld := last.Date()
	switch today.Sub(time.Date(ly, lm, ld, 0, 0, 0, 0, time.UTC)) {
	case 0:
		return today, max(current, 1)
	case 24 * time.Hour:
		return today, current + 1
	}
	return today, 1
}

// condition is an achievement_definitions.condition.
type condition struct {
	Type   string `json:"type"`
	Count  int64  `json:"count"`
	Days   int32  `json:"days"`
	Course string `json:"course"`
}

func awardAchievements(ctx context.Context, q *dbgen.Queries, c Completion, completed, streak int32) ([]Award, error) {
	defs, err := q.ListAchievementConditions(ctx)
	if err != nil {
		return nil, err
	}
	// Counts are loaded only if some definition needs them.
	var courses []string
	var coursesLoaded bool
	var hintFree, firstTry *int64
	var awards []Award
	for _, d := range defs {
		var cond condition
		if err := json.Unmarshal(d.Condition, &cond); err != nil {
			return nil, fmt.Errorf("achievement %s: %w", d.Key, err)
		}
		earned := false
		switch cond.Type {
		case "lessons_completed":
			earned = int64(completed) >= cond.Count
		case "streak":
			earned = streak >= cond.Days
		case "courses_completed", "course_completed":
			if !coursesLoaded {
				if courses, err = q.CompletedCourseSlugs(ctx, c.UserID); err != nil {
					return nil, err
				}
				coursesLoaded = true
			}
			if cond.Type == "courses_completed" {
				earned = int64(len(courses)) >= cond.Count
			} else {
				earned = slices.Contains(courses, cond.Course)
			}
		case "lessons_without_hints":
			if hintFree == nil {
				n, err := q.CountHintFreePasses(ctx, c.UserID)
				if err != nil {
					return nil, err
				}
				hintFree = &n
			}
			earned = *hintFree >= cond.Count
		case "first_try_passes":
			if firstTry == nil {
				n, err := q.CountFirstTryPasses(ctx, c.UserID)
				if err != nil {
					return nil, err
				}
				firstTry = &n
			}
			earned = *firstTry >= cond.Count
		default:
			continue // a condition this version doesn't know is never awarded by mistake
		}
		if !earned {
			continue
		}
		n, err := q.AwardAchievement(ctx, dbgen.AwardAchievementParams{UserID: c.UserID, AchievementKey: d.Key})
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue // already had it
		}
		awards = append(awards, Award{Key: d.Key, Title: d.Title})
		if c.Activity {
			key := d.Key
			if err := q.InsertActivity(ctx, dbgen.InsertActivityParams{UserID: c.UserID, Type: "achievement-earned", AchievementKey: &key}); err != nil {
				return nil, err
			}
		}
	}
	return awards, nil
}
