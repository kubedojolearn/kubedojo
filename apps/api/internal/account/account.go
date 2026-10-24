// Package account serves the opt-in leaderboard, the learner's data export and account deletion
// (requirements PR2–PR5).
package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// ErasureDelay is how long a deleted account is kept (tombstoned, invisible and unlinked) before
// every remaining row is erased. It matches the privacy notice and the deletion API description.
const ErasureDelay = 30 * 24 * time.Hour

// ErrHandleMismatch means the typed confirmation didn't match the account's handle.
var ErrHandleMismatch = errors.New("confirmation handle does not match")

// LeaderboardEntry mirrors the OpenAPI LeaderboardEntry.
type LeaderboardEntry struct {
	Rank             int64   `json:"rank"`
	Handle           string  `json:"handle"`
	DisplayName      string  `json:"displayName"`
	AvatarURL        *string `json:"avatarUrl"`
	TotalXP          int32   `json:"totalXp"`
	CompletedLessons int32   `json:"completedLessons"`
}

// LeaderboardCursor is the keyset position after the last entry of a page.
type LeaderboardCursor struct {
	XP     int32     `json:"x"`
	UserID uuid.UUID `json:"u"`
}

// Service reads and changes account-level data.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns a Service backed by pool.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Leaderboard returns one page of opted-in learners, highest XP first.
func (s *Service) Leaderboard(ctx context.Context, after *LeaderboardCursor, limit int32) ([]LeaderboardEntry, *LeaderboardCursor, error) {
	params := dbgen.ListLeaderboardParams{PageLimit: limit + 1}
	if after != nil {
		params.AfterXp, params.AfterUserID = &after.XP, &after.UserID
	}
	rows, err := dbgen.New(s.pool).ListLeaderboard(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("leaderboard: %w", err)
	}
	var next *LeaderboardCursor
	if len(rows) > int(limit) {
		last := rows[limit-1]
		next = &LeaderboardCursor{XP: last.TotalXp, UserID: last.UserID}
		rows = rows[:limit]
	}
	out := make([]LeaderboardEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, LeaderboardEntry{
			Rank: r.Rank, Handle: r.Handle, DisplayName: r.DisplayName, AvatarURL: r.AvatarUrl,
			TotalXP: r.TotalXp, CompletedLessons: r.CompletedLessons,
		})
	}
	return out, next, nil
}

// Export returns everything stored about the user as one JSON document.
func (s *Service) Export(ctx context.Context, userID uuid.UUID) (json.RawMessage, error) {
	doc, err := dbgen.New(s.pool).ExportUserData(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	return doc, nil
}

// RequestDeletion tombstones the account, unlinks its GitHub identity, hides it from the
// leaderboard and profiles, and ends its labs, in one transaction. confirmHandle must match the
// account's handle (case-insensitive). The rest is erased by EraseDue after ErasureDelay.
func (s *Service) RequestDeletion(ctx context.Context, userID uuid.UUID, confirmHandle string) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if _, err := q.TombstoneUser(ctx, dbgen.TombstoneUserParams{ID: userID, ConfirmHandle: confirmHandle}); errors.Is(err, pgx.ErrNoRows) {
			return ErrHandleMismatch
		} else if err != nil {
			return err
		}
		if err := q.UnlinkIdentities(ctx, userID); err != nil {
			return err
		}
		if err := q.HideUser(ctx, userID); err != nil {
			return err
		}
		if err := q.EndUserLabs(ctx, userID); err != nil {
			return err
		}
		_, err := q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
			ActorUserID: &userID, ActorKind: "user", Action: "user.deletion_requested",
			EntityType: "user", EntityID: userID.String(), Details: []byte("{}"),
		})
		return err
	})
}

// EraseDue hard-deletes up to batch accounts that were deleted before now-ErasureDelay. Each
// account is erased in its own transaction, so one failure doesn't block the rest.
func (s *Service) EraseDue(ctx context.Context, now time.Time, batch int32) (int, error) {
	ids, err := dbgen.New(s.pool).ListUsersDueForErasure(ctx, dbgen.ListUsersDueForErasureParams{Cutoff: now.Add(-ErasureDelay), Batch: batch})
	if err != nil {
		return 0, fmt.Errorf("list accounts due for erasure: %w", err)
	}
	erased := 0
	var errs []error
	for _, id := range ids {
		err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			if err := q.AllowUserErasure(ctx, id.String()); err != nil {
				return err
			}
			n, err := q.EraseUser(ctx, id)
			if err != nil || n == 0 {
				return err
			}
			erased++
			_, err = q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
				ActorKind: "system", Action: "user.erased", EntityType: "user", EntityID: id.String(), Details: []byte("{}"),
			})
			return err
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("erase %s: %w", id, err))
		}
	}
	return erased, errors.Join(errs...)
}
