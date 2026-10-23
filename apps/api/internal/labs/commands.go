package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Stored output limits (command_executions CHECKs).
const (
	maxStdout = 256 << 10
	maxStderr = 64 << 10
	// storedEventLimit matches the events a lab keeps (simulator.DefaultLimits.MaxEvents).
	storedEventLimit int32 = 500
)

// ErrInvalidInput means the command text can't be stored (NUL bytes or invalid UTF-8).
var ErrInvalidInput = errors.New("command input must be UTF-8 text without NUL characters")

// CommandResult mirrors the OpenAPI CommandResult.
type CommandResult struct {
	RequestID       uuid.UUID `json:"requestId"`
	Sequence        int64     `json:"sequence"`
	Input           string    `json:"input"`
	Stdout          string    `json:"stdout"`
	Stderr          string    `json:"stderr"`
	ExitCode        int16     `json:"exitCode"`
	ErrorCode       *string   `json:"errorCode"`
	Mutated         bool      `json:"mutated"`
	SnapshotVersion int64     `json:"snapshotVersion"`
	DurationMs      int32     `json:"durationMs"`
	ExecutedAt      time.Time `json:"executedAt"`
}

func commandFromRow(r dbgen.CommandExecution) CommandResult {
	return CommandResult{
		RequestID: r.RequestID, Sequence: r.Sequence, Input: r.Input, Stdout: r.Stdout, Stderr: r.Stderr,
		ExitCode: r.ExitCode, ErrorCode: r.ErrorCode, Mutated: r.Mutated, SnapshotVersion: r.SnapshotVersion,
		DurationMs: r.DurationMs, ExecutedAt: r.ExecutedAt,
	}
}

// ValidInput reports whether input can be stored as command text.
func ValidInput(input string) bool {
	if input == "" || len(input) > kubectl.MaxInputBytes || !utf8.ValidString(input) {
		return false
	}
	for i := 0; i < len(input); i++ {
		if input[i] == 0 {
			return false
		}
	}
	return true
}

func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	note := fmt.Sprintf("\n… output truncated at %d KiB\n", limit>>10)
	return truncate(s, limit-len(note)) + note
}

// storedEvents loads the lab's persisted events for commands that show them.
func storedEvents(ctx context.Context, q *dbgen.Queries, labID uuid.UUID) ([]simulator.Event, error) {
	rows, err := q.ListLabEvents(ctx, dbgen.ListLabEventsParams{SessionID: labID, AfterSeq: 0, PageLimit: storedEventLimit})
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	out := make([]simulator.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, simulator.Event{
			Type: r.Type, Reason: r.Reason, Kind: r.InvolvedKind, Namespace: r.InvolvedNamespace,
			Name: r.InvolvedName, Message: r.Message, Time: r.OccurredAt,
		})
	}
	return out, nil
}

// RunCommand executes one terminal command at most once per requestID. A repeated requestID
// returns the stored result, whatever the input. Only read commands exist in this version, so
// commands never change lab state; they are recorded in the lab's history (with Secret values
// redacted) and count as activity.
func (sv *Service) RunCommand(ctx context.Context, userID, labID, requestID uuid.UUID, input string) (CommandResult, error) {
	if !ValidInput(input) {
		return CommandResult{}, ErrInvalidInput
	}
	q := dbgen.New(sv.pool)
	if _, err := q.GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return CommandResult{}, ErrNotFound
	} else if err != nil {
		return CommandResult{}, err
	}
	if prev, err := q.GetCommandByRequestID(ctx, dbgen.GetCommandByRequestIDParams{SessionID: labID, RequestID: requestID}); err == nil {
		return commandFromRow(prev), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CommandResult{}, err
	}

	var result CommandResult
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		// A retry may have committed while this request waited for the lab lock.
		if prev, err := q.GetCommandByRequestID(ctx, dbgen.GetCommandByRequestIDParams{SessionID: s.ID, RequestID: requestID}); err == nil {
			result = commandFromRow(prev)
			return Change{}, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Change{}, err
		}
		f, err := sv.fixtures.get(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return Change{}, err
		}
		state, err := LoadState(ctx, q, s, f)
		if err != nil {
			return Change{}, err
		}
		events, err := storedEvents(ctx, q, s.ID)
		if err != nil {
			return Change{}, err
		}

		started := time.Now()
		res := kubectl.Plan(input).Run(kubectl.Env{State: state, Events: events})
		duration := time.Since(started).Milliseconds()

		seq, err := q.NextCommandSeq(ctx, s.ID)
		if err != nil {
			return Change{}, err
		}
		var parsed []byte
		if res.Parsed != nil {
			if parsed, err = json.Marshal(res.Parsed); err != nil {
				return Change{}, err
			}
		}
		var code *string
		if res.ErrorCode != "" {
			c := string(res.ErrorCode)
			code = &c
		}
		stderr := clip(res.Stderr, maxStderr)
		row, err := q.InsertCommandExecution(ctx, dbgen.InsertCommandExecutionParams{
			SessionID: s.ID, Sequence: seq, RequestID: requestID, Input: input, Parsed: parsed,
			Stdout: clip(res.Redacted, maxStdout), Stderr: stderr, ExitCode: int16(min(max(res.ExitCode, 0), 255)), //nolint:gosec // clamped to 0-255
			ErrorCode: code, Mutated: false, SnapshotVersion: s.Version, DurationMs: int32(min(duration, math.MaxInt32)), //nolint:gosec // clamped to int32
		})
		if err != nil {
			return Change{}, fmt.Errorf("record command: %w", err)
		}
		result = commandFromRow(row)
		// The learner sees their own Secret values; history keeps the redacted copy.
		result.Stdout = clip(res.Stdout, maxStdout)
		return Change{}, nil
	})
	return result, err
}

// CommandCursor is the keyset position in the command history.
type CommandCursor struct {
	Sequence int64 `json:"s"`
}

// Commands returns the lab's command history, newest first.
func (sv *Service) Commands(ctx context.Context, userID, labID uuid.UUID, before *CommandCursor, limit int32) ([]CommandResult, *CommandCursor, error) {
	q := dbgen.New(sv.pool)
	if _, err := q.GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	} else if err != nil {
		return nil, nil, err
	}
	params := dbgen.ListCommandsParams{SessionID: labID, PageLimit: limit + 1}
	if before != nil {
		params.BeforeSeq = &before.Sequence
	}
	rows, err := q.ListCommands(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	var next *CommandCursor
	if len(rows) > int(limit) {
		next = &CommandCursor{Sequence: rows[limit-1].Sequence}
		rows = rows[:limit]
	}
	out := make([]CommandResult, 0, len(rows))
	for _, r := range rows {
		out = append(out, commandFromRow(r))
	}
	return out, next, nil
}
