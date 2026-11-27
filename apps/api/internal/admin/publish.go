package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/grading"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Domain errors for admin publishing operations.
var (
	ErrNotFound = errors.New("challenge version not found")
	ErrConflict = errors.New("conflict")
)

// ValidationError mirrors the error item in OpenAPI ContentValidationReport.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SolutionReplay mirrors OpenAPI ContentValidationReport.solutionReplay.
type SolutionReplay struct {
	Passed bool             `json:"passed"`
	Goals  []grading.Result `json:"goals"`
}

// ValidationReport mirrors OpenAPI ContentValidationReport.
type ValidationReport struct {
	ChallengeVersionID uuid.UUID         `json:"challengeVersionId"`
	Valid              bool              `json:"valid"`
	Errors             []ValidationError `json:"errors"`
	SolutionReplay     SolutionReplay    `json:"solutionReplay"`
}

type storedSpec struct {
	Challenge struct {
		Goals        []grading.Goal `json:"goals"`
		InitialFiles []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"initialFiles"`
		Solution struct {
			Files []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
			Commands []string          `json:"commands"`
			Answers  map[string]string `json:"answers"`
		} `json:"solution"`
	} `json:"challenge"`
	Fixture simulator.Fixture `json:"fixture"`
}

// Validate re-validates an imported challenge version against the current simulator and replays
// its hidden reference solution.
func (s *Service) Validate(ctx context.Context, versionID uuid.UUID) (ValidationReport, error) {
	row, err := dbgen.New(s.pool).GetAdminChallengeVersion(ctx, versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ValidationReport{}, ErrNotFound
	}
	if err != nil {
		return ValidationReport{}, fmt.Errorf("load challenge version: %w", err)
	}

	report := ValidationReport{
		ChallengeVersionID: row.ID,
		Errors:             []ValidationError{},
		SolutionReplay:     SolutionReplay{Goals: []grading.Result{}},
	}
	if row.SimulatorVersion != simulator.Version {
		report.Errors = append(report.Errors, ValidationError{
			Path:    "simulatorVersion",
			Message: fmt.Sprintf("version targets %q; current simulator is %q", row.SimulatorVersion, simulator.Version),
		})
	}

	var spec storedSpec
	if err := json.Unmarshal(row.Spec, &spec); err != nil {
		report.Errors = append(report.Errors, ValidationError{Path: "spec", Message: fmt.Sprintf("invalid JSON: %v", err)})
		return report, nil
	}
	if len(spec.Challenge.Goals) == 0 {
		report.Errors = append(report.Errors, ValidationError{Path: "challenge.goals", Message: "challenge has no goals"})
		return report, nil
	}

	st, err := simulator.Load(spec.Fixture, 42)
	if err != nil {
		report.Errors = append(report.Errors, ValidationError{Path: "fixture", Message: fmt.Sprintf("load fixture: %v", err)})
		return report, nil
	}
	events := append([]simulator.Event{}, st.Events()...)
	restored := simulator.NewState(st.Now, st.Gen.Seed, st.Images)
	if err := restored.Restore(st.All()); err != nil {
		report.Errors = append(report.Errors, ValidationError{Path: "fixture", Message: fmt.Sprintf("restore fixture: %v", err)})
		return report, nil
	}
	restored.Gen, restored.Revision, restored.CurrentNamespace = st.Gen, st.Revision, st.CurrentNamespace
	files := map[string]string{}
	for _, f := range spec.Challenge.InitialFiles {
		files[f.Path] = f.Content
	}
	env := kubectl.Env{State: restored, Events: events, Files: files}

	before := grading.Evaluate(env.State, spec.Challenge.Goals, grading.Inputs{Answers: spec.Challenge.Solution.Answers})
	if grading.Score(before) == 100 && !onlyAnswerGoals(spec.Challenge.Goals) {
		report.Errors = append(report.Errors, ValidationError{
			Path:    "challenge.goals",
			Message: "every goal already passes before any learner action",
		})
	}

	for _, f := range spec.Challenge.Solution.Files {
		env.Files[f.Path] = f.Content
	}
	var commands []kubectl.Parsed
	for i, cmd := range spec.Challenge.Solution.Commands {
		r := kubectl.Execute(cmd, env)
		if r.Err != nil {
			report.Errors = append(report.Errors, ValidationError{
				Path:    fmt.Sprintf("challenge.solution.commands[%d]", i),
				Message: fmt.Sprintf("simulator error: %v", r.Err),
			})
			continue
		}
		if r.ErrorCode != "" {
			report.Errors = append(report.Errors, ValidationError{
				Path:    fmt.Sprintf("challenge.solution.commands[%d]", i),
				Message: fmt.Sprintf("%s (exit %d): %s", r.ErrorCode, r.ExitCode, strings.TrimSpace(r.Stderr)),
			})
		}
		if r.ExitCode == 0 && r.Parsed != nil {
			commands = append(commands, *r.Parsed)
		}
	}

	answers := map[string]string{}
	for k, v := range spec.Challenge.Solution.Answers {
		answers[k] = v
	}
	for _, g := range spec.Challenge.Goals {
		if g.Type == "answer" && g.Expected != nil && g.Expected.Static == nil {
			want, err := grading.ExpectedAnswer(env.State, g)
			if err != nil {
				report.Errors = append(report.Errors, ValidationError{
					Path:    "challenge.goals." + g.ID,
					Message: err.Error(),
				})
				continue
			}
			answers[g.ID] = want
		}
	}

	results := grading.Evaluate(env.State, spec.Challenge.Goals, grading.Inputs{Answers: answers, Commands: commands})
	allPassed := len(results) > 0
	for _, r := range results {
		if !r.Passed {
			allPassed = false
			report.Errors = append(report.Errors, ValidationError{
				Path:    "challenge.goals." + r.GoalID,
				Message: r.Message,
			})
		}
	}
	report.SolutionReplay = SolutionReplay{Passed: allPassed, Goals: results}
	report.Valid = len(report.Errors) == 0 && allPassed
	return report, nil
}

// Publish publishes or unpublishes an imported challenge version in one transaction and records
// an audit entry attributed to actorUserID.
func (s *Service) Publish(ctx context.Context, actorUserID, versionID uuid.UUID, action string) (LessonContent, error) {
	if action != "publish" && action != "unpublish" {
		return LessonContent{}, fmt.Errorf("%w: action must be publish or unpublish", ErrConflict)
	}
	var out LessonContent
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		row, err := q.GetAdminChallengeVersion(ctx, versionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("load challenge version: %w", err)
		}
		lesson, err := q.LockLessonForPublish(ctx, row.LessonID)
		if err != nil {
			return fmt.Errorf("lock lesson: %w", err)
		}
		lessonKey := row.CourseSlug + "/" + row.LessonSlug
		sha := strings.TrimSpace(row.ContentSha256)
		details, _ := json.Marshal(map[string]any{"lesson": lessonKey, "version": row.Version, "sha256": sha})

		switch action {
		case "publish":
			if row.SimulatorVersion != simulator.Version {
				return fmt.Errorf("%w: challenge version targets simulator %q (current is %q)", ErrConflict, row.SimulatorVersion, simulator.Version)
			}
			if lesson.CurrentChallengeVersionID == nil || *lesson.CurrentChallengeVersionID != versionID {
				if err := q.ActivateChallengeVersion(ctx, versionID); err != nil {
					return fmt.Errorf("activate version: %w", err)
				}
				if err := q.SetLessonCurrentVersion(ctx, dbgen.SetLessonCurrentVersionParams{
					LessonID:           row.LessonID,
					ChallengeVersionID: &versionID,
				}); err != nil {
					return fmt.Errorf("set current version: %w", err)
				}
				if err := q.SetCoursePublished(ctx, dbgen.SetCoursePublishedParams{ID: lesson.CourseID, Published: true}); err != nil {
					return fmt.Errorf("publish course: %w", err)
				}
				var oldRef *string
				if prev := lesson.CurrentChallengeVersionID; prev != nil {
					if err := q.RetireChallengeVersion(ctx, *prev); err != nil {
						return fmt.Errorf("retire previous version: %w", err)
					}
					s := prev.String()
					oldRef = &s
				}
				newRef := versionID.String()
				if _, err := q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
					ActorUserID: &actorUserID, ActorKind: "user", Action: "content.published",
					EntityType: "lesson", EntityID: lessonKey, OldRef: oldRef, NewRef: &newRef, Details: details,
				}); err != nil {
					return fmt.Errorf("audit publish: %w", err)
				}
			}

		case "unpublish":
			if lesson.CurrentChallengeVersionID == nil || *lesson.CurrentChallengeVersionID != versionID {
				return fmt.Errorf("%w: challenge version is not currently published", ErrConflict)
			}
			if err := q.SetLessonCurrentVersion(ctx, dbgen.SetLessonCurrentVersionParams{
				LessonID:           row.LessonID,
				ChallengeVersionID: nil,
			}); err != nil {
				return fmt.Errorf("clear current version: %w", err)
			}
			if err := q.RetireChallengeVersion(ctx, versionID); err != nil {
				return fmt.Errorf("retire version: %w", err)
			}
			oldRef := versionID.String()
			if _, err := q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
				ActorUserID: &actorUserID, ActorKind: "user", Action: "content.unpublished",
				EntityType: "lesson", EntityID: lessonKey, OldRef: &oldRef, NewRef: nil, Details: details,
			}); err != nil {
				return fmt.Errorf("audit unpublish: %w", err)
			}
		}

		rows, err := q.ListAdminLessonContent(ctx, row.LessonID)
		if err != nil {
			return fmt.Errorf("reload lesson content: %w", err)
		}
		out = lessonContentFromRows(rows)
		return nil
	})
	return out, err
}

func lessonContentFromRows(rows []dbgen.ListAdminLessonContentRow) LessonContent {
	var out LessonContent
	out.Versions = []ChallengeVersion{}
	for i, r := range rows {
		if i == 0 {
			out.LessonID = r.CourseSlug + "/" + r.LessonSlug
			out.Title = r.Title
			out.CurrentVersionID = r.CurrentChallengeVersionID
		}
		if r.VersionID == nil {
			continue
		}
		out.Versions = append(out.Versions, ChallengeVersion{
			ID: *r.VersionID, Version: *r.Version, ContentSHA256: strings.TrimSpace(*r.ContentSha256),
			Status: *r.Status, ImportedAt: *r.ImportedAt, PublishedAt: r.PublishedAt,
		})
	}
	return out
}

func onlyAnswerGoals(goals []grading.Goal) bool {
	for _, g := range goals {
		if g.Type != "answer" {
			return false
		}
	}
	return true
}
