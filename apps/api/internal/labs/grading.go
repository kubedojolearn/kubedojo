package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/gamification"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/grading"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// Grading (ADR-0007): learners answer answer-type goals, reveal hints in order, and ask for a
// check. A check grades every goal against the lab as it is; if all pass, the lab completes
// and the lesson's XP is awarded once.

// OpValidate is the idempotency operation of POST /labs/{id}/validate.
const OpValidate = "lab.validate"

// ReasonCompleted is lab_sessions.state_reason for a lab whose goals all passed.
const ReasonCompleted = "GOALS_PASSED"

// Grading errors.
var (
	// ErrGoalNotFound: no answer-type goal with that ID in this lab's challenge.
	ErrGoalNotFound = errors.New("goal not found")
	// ErrInvalidAnswer: empty, too long, not a number for a number goal, or not one of the choices.
	ErrInvalidAnswer = errors.New("invalid answer")
	// ErrNoSubmissionsLeft: the goal's submission limit is used up.
	ErrNoSubmissionsLeft = errors.New("no submissions left")
	// ErrHintNotFound: the challenge has no hint at that index.
	ErrHintNotFound = errors.New("hint not found")
	// ErrHintOrder: hints must be revealed in order.
	ErrHintOrder = errors.New("reveal the earlier hints first")
)

// ValidationResult mirrors the OpenAPI ValidationResult.
type ValidationResult struct {
	AttemptID    uuid.UUID            `json:"attemptId"`
	Passed       bool                 `json:"passed"`
	Score        int                  `json:"score"`
	XPAwarded    int32                `json:"xpAwarded"`
	Goals        []grading.Result     `json:"goals"`
	LabState     string               `json:"labState"`
	EvaluatedAt  time.Time            `json:"evaluatedAt"`
	Achievements []gamification.Award `json:"achievements"`
}

// AnswerStatus mirrors the OpenAPI AnswerStatus.
type AnswerStatus struct {
	GoalID               string  `json:"goalId"`
	Submissions          int     `json:"submissions"`
	SubmissionsRemaining int     `json:"submissionsRemaining"`
	LastAnswer           *string `json:"lastAnswer"`
}

// Hint mirrors the OpenAPI Hint.
type Hint struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	PenaltyXP int32  `json:"penaltyXp"`
	HintsUsed int    `json:"hintsUsed"`
	HintCount int    `json:"hintCount"`
}

// LabGoals mirrors the OpenAPI LabGoals: what the goals panel shows after a reload.
type LabGoals struct {
	Answers     []AnswerStatus    `json:"answers"`
	Hints       []Hint            `json:"hints"`
	HintCount   int               `json:"hintCount"`
	PenaltyXP   int32             `json:"hintPenaltyXp"`
	LastAttempt *ValidationResult `json:"lastAttempt"`
}

func answerGoal(def definition, goalID string) (grading.Goal, bool) {
	for _, g := range def.Goals {
		if g.ID == goalID && g.Type == "answer" {
			return g, true
		}
	}
	return grading.Goal{}, false
}

func answerStatuses(def definition, rows []dbgen.LatestAnswersRow) []AnswerStatus {
	byGoal := map[string]dbgen.LatestAnswersRow{}
	for _, r := range rows {
		byGoal[r.GoalID] = r
	}
	out := []AnswerStatus{}
	for _, g := range def.Goals {
		if g.Type != "answer" {
			continue
		}
		st := AnswerStatus{GoalID: g.ID, SubmissionsRemaining: grading.MaxSubmissions(g)}
		if r, ok := byGoal[g.ID]; ok {
			answer := r.Answer
			st.Submissions, st.LastAnswer = int(r.SubmissionNo), &answer
			st.SubmissionsRemaining = max(grading.MaxSubmissions(g)-st.Submissions, 0)
		}
		out = append(out, st)
	}
	return out
}

// validAnswer checks an answer's shape before it uses up a submission.
func validAnswer(g grading.Goal, answer string) bool {
	answer = strings.TrimSpace(answer)
	if answer == "" || len(answer) > 256 || !utf8.ValidString(answer) || strings.ContainsRune(answer, 0) {
		return false
	}
	switch g.Input {
	case "number":
		_, err := strconv.ParseFloat(answer, 64)
		return err == nil
	case "choice":
		return slices.Contains(g.Choices, answer)
	}
	return true
}

// SubmitAnswer stores an answer for an answer goal. Whether it is right is only reported by a
// check, so answers can't be probed one at a time without spending checks too.
func (sv *Service) SubmitAnswer(ctx context.Context, userID, labID uuid.UUID, goalID, answer string) (AnswerStatus, error) {
	var out AnswerStatus
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		def, err := sv.fixtures.definition(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return Change{}, err
		}
		g, ok := answerGoal(def, goalID)
		if !ok {
			return Change{}, ErrGoalNotFound
		}
		if !validAnswer(g, answer) {
			return Change{}, ErrInvalidAnswer
		}
		rows, err := q.LatestAnswers(ctx, s.ID)
		if err != nil {
			return Change{}, err
		}
		used := 0
		for _, r := range rows {
			if r.GoalID == goalID {
				used = int(r.SubmissionNo)
			}
		}
		limit := grading.MaxSubmissions(g)
		if used >= limit {
			return Change{}, ErrNoSubmissionsLeft
		}
		answer = strings.TrimSpace(answer)
		if err := q.InsertAnswer(ctx, dbgen.InsertAnswerParams{SessionID: s.ID, GoalID: goalID, SubmissionNo: int16(used + 1), Answer: answer}); err != nil { //nolint:gosec // at most 10
			return Change{}, err
		}
		out = AnswerStatus{GoalID: goalID, Submissions: used + 1, SubmissionsRemaining: limit - used - 1, LastAnswer: &answer}
		return Change{}, nil
	})
	return out, err
}

// RevealHint returns hint index. Hints are revealed in order; revealing one again costs nothing.
func (sv *Service) RevealHint(ctx context.Context, userID, labID uuid.UUID, index int) (Hint, error) {
	var out Hint
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		def, err := sv.fixtures.definition(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return Change{}, err
		}
		used := int(s.HintsUsed)
		switch {
		case index < 0 || index >= len(def.Hints):
			return Change{}, ErrHintNotFound
		case index > used:
			return Change{}, ErrHintOrder
		case index == used:
			if err := q.RecordHint(ctx, dbgen.RecordHintParams{SessionID: s.ID, HintIndex: int16(index)}); err != nil { //nolint:gosec // at most 9
				return Change{}, err
			}
			used++
			if err := q.SetHintsUsed(ctx, dbgen.SetHintsUsedParams{ID: s.ID, HintsUsed: int16(used)}); err != nil { //nolint:gosec // at most 10
				return Change{}, err
			}
			optOut, err := q.GetAnalyticsOptOut(ctx, userID)
			if err != nil {
				return Change{}, err
			}
			if !optOut {
				lesson := s.LessonID
				if err := q.InsertActivity(ctx, dbgen.InsertActivityParams{UserID: userID, Type: "hint-revealed", LessonID: &lesson}); err != nil {
					return Change{}, err
				}
			}
		}
		out = Hint{Index: index, Text: def.Hints[index], PenaltyXP: def.Reward.HintPenaltyPerHintXP, HintsUsed: used, HintCount: len(def.Hints)}
		return Change{}, nil
	})
	return out, err
}

// lessonXP is the award for passing: base, plus the bonus on the learner's first check of the
// lesson, minus the penalty for each hint revealed in this lab; never below zero.
func lessonXP(r Reward, firstTry bool, hints int) int32 {
	xp := r.BaseXP - int32(hints)*r.HintPenaltyPerHintXP //nolint:gosec // hints ≤ 10
	if firstTry {
		xp += r.FirstTryBonusXP
	}
	return max(xp, 0)
}

// Validate grades the lab. With all goals passing, the lab completes and the lesson's XP and
// achievements are awarded; a retry with the same key returns the stored result.
func (sv *Service) Validate(ctx context.Context, userID, labID uuid.UUID, key string) (ValidationResult, error) {
	if res, err := replayValidation(ctx, dbgen.New(sv.pool), userID, labID, key); err != nil || res != nil {
		return derefResult(res), err
	}
	var result ValidationResult
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		if res, err := replayValidation(ctx, q, userID, labID, key); err != nil || res != nil {
			result = derefResult(res)
			return Change{}, err
		}
		def, err := sv.fixtures.definition(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return Change{}, err
		}
		state, err := LoadState(ctx, q, s, def.Fixture)
		if err != nil {
			return Change{}, err
		}
		in, err := gradingInputs(ctx, q, s.ID)
		if err != nil {
			return Change{}, err
		}
		results := grading.Evaluate(state, def.Goals, in)
		score := grading.Score(results)
		passed := score == 100

		previous, err := q.CountLessonAttempts(ctx, dbgen.CountLessonAttemptsParams{UserID: userID, LessonID: s.LessonID})
		if err != nil {
			return Change{}, err
		}
		goalJSON, err := json.Marshal(results)
		if err != nil {
			return Change{}, err
		}
		attempt, err := q.InsertAttempt(ctx, dbgen.InsertAttemptParams{
			SessionID: s.ID, UserID: userID, ChallengeVersionID: s.ChallengeVersionID, IdempotencyKey: key,
			Passed: passed, Score: int16(score), HintsUsed: s.HintsUsed, GoalResults: goalJSON, //nolint:gosec // 0-100
		})
		if err != nil {
			return Change{}, fmt.Errorf("record attempt: %w", err)
		}
		result = ValidationResult{
			AttemptID: attempt.ID, Passed: passed, Score: score, Goals: results, LabState: "active",
			EvaluatedAt: attempt.EvaluatedAt, Achievements: []gamification.Award{},
		}
		if passed {
			optOut, err := q.GetAnalyticsOptOut(ctx, userID)
			if err != nil {
				return Change{}, err
			}
			outcome, err := gamification.CompleteLesson(ctx, q, gamification.Completion{
				LessonID: s.LessonID, UserID: userID, AttemptID: attempt.ID, Score: int16(score), //nolint:gosec // 100
				XP: lessonXP(def.Reward, previous == 0, int(s.HintsUsed)), Activity: !optOut, Now: sv.now(),
			})
			if err != nil {
				return Change{}, err
			}
			result.XPAwarded = outcome.XPAwarded
			if outcome.Achievements != nil {
				result.Achievements = outcome.Achievements
			}
			reason := ReasonCompleted
			if err := q.EndLab(ctx, dbgen.EndLabParams{ID: s.ID, State: "completed", StateReason: &reason}); err != nil {
				return Change{}, err
			}
			result.LabState = "completed"
		} else if err := q.RecordBestScore(ctx, dbgen.RecordBestScoreParams{UserID: userID, LessonID: s.LessonID, Score: int16(score)}); err != nil { //nolint:gosec // 0-100
			return Change{}, err
		}
		body, err := json.Marshal(result)
		if err != nil {
			return Change{}, err
		}
		err = q.InsertIdempotencyRecord(ctx, dbgen.InsertIdempotencyRecordParams{
			UserID: userID, Operation: OpValidate, Key: key, TargetID: labID, StatusCode: http.StatusOK, Response: body,
		})
		if database.IsCode(err, database.CodeUniqueViolation) {
			return Change{}, ErrIdempotencyConflict
		}
		return Change{}, err
	})
	if errors.Is(err, ErrNotActive) {
		// A retry of the check that completed the lab finds it ended: answer from the record.
		if res, rerr := replayValidation(ctx, dbgen.New(sv.pool), userID, labID, key); rerr != nil || res != nil {
			return derefResult(res), rerr
		}
	}
	return result, err
}

func replayValidation(ctx context.Context, q *dbgen.Queries, userID, labID uuid.UUID, key string) (*ValidationResult, error) {
	rec, err := q.GetIdempotencyRecord(ctx, dbgen.GetIdempotencyRecordParams{UserID: userID, Operation: OpValidate, Key: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rec.TargetID != labID {
		return nil, ErrIdempotencyConflict
	}
	var res ValidationResult
	if err := json.Unmarshal(rec.Response, &res); err != nil {
		return nil, fmt.Errorf("stored validation: %w", err)
	}
	return &res, nil
}

func derefResult(r *ValidationResult) ValidationResult {
	if r == nil {
		return ValidationResult{}
	}
	return *r
}

// gradingInputs loads the learner's answers and successful commands.
func gradingInputs(ctx context.Context, q *dbgen.Queries, labID uuid.UUID) (grading.Inputs, error) {
	in := grading.Inputs{Answers: map[string]string{}}
	answers, err := q.LatestAnswers(ctx, labID)
	if err != nil {
		return in, err
	}
	for _, a := range answers {
		in.Answers[a.GoalID] = a.Answer
	}
	rows, err := q.ListObservedCommands(ctx, labID)
	if err != nil {
		return in, err
	}
	for _, raw := range rows {
		var p kubectl.Parsed
		if err := json.Unmarshal(raw, &p); err == nil {
			in.Commands = append(in.Commands, p)
		}
	}
	return in, nil
}

// Goals returns what the goals panel needs after a reload: answer counts, revealed hints and
// the latest check. It works on ended labs too, so a completed lab still shows its result.
func (sv *Service) Goals(ctx context.Context, userID, labID uuid.UUID) (LabGoals, error) {
	var out LabGoals
	err := sv.readTx(ctx, func(q *dbgen.Queries) error {
		s, err := q.GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		def, err := sv.fixtures.definition(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return err
		}
		answers, err := q.LatestAnswers(ctx, s.ID)
		if err != nil {
			return err
		}
		out = LabGoals{
			Answers: answerStatuses(def, answers), Hints: []Hint{}, HintCount: len(def.Hints),
			PenaltyXP: def.Reward.HintPenaltyPerHintXP,
		}
		for i := 0; i < int(s.HintsUsed) && i < len(def.Hints); i++ {
			out.Hints = append(out.Hints, Hint{Index: i, Text: def.Hints[i], PenaltyXP: def.Reward.HintPenaltyPerHintXP, HintsUsed: int(s.HintsUsed), HintCount: len(def.Hints)})
		}
		a, err := q.LatestAttempt(ctx, s.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		res := ValidationResult{AttemptID: a.ID, Passed: a.Passed, Score: int(a.Score), LabState: s.State, EvaluatedAt: a.EvaluatedAt, Achievements: []gamification.Award{}}
		if err := json.Unmarshal(a.GoalResults, &res.Goals); err != nil {
			return fmt.Errorf("stored goal results: %w", err)
		}
		if res.XPAwarded, err = q.XPForAttempt(ctx, &a.ID); err != nil {
			return err
		}
		out.LastAttempt = &res
		return nil
	})
	return out, err
}
