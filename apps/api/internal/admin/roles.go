// Package admin holds operator and admin operations. Role changes happen only here, via the
// operator CLI — never through a public API (threat T17).
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// ErrUserNotFound is returned when no active user has the handle.
var ErrUserNotFound = errors.New("no active user with that handle")

// SetRole changes a user's role and writes an audit entry in the same transaction.
func SetRole(ctx context.Context, pool *pgxpool.Pool, handle string, role auth.Role, reason string) (uuid.UUID, auth.Role, error) {
	if role != auth.RoleAdmin && role != auth.RoleLearner {
		return uuid.Nil, "", fmt.Errorf("unknown role %q", role)
	}
	var userID uuid.UUID
	var oldRole auth.Role
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		row, err := q.SetRoleByHandle(ctx, dbgen.SetRoleByHandleParams{Handle: handle, Role: string(role)})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}
		userID, oldRole = row.ID, auth.Role(row.OldRole)
		details, _ := json.Marshal(map[string]string{"reason": reason})
		oldRef, newRef := string(oldRole), string(role)
		_, err = q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
			ActorKind: "operator", Action: "user.role_changed", EntityType: "user", EntityID: row.ID.String(),
			OldRef: &oldRef, NewRef: &newRef, Details: details,
		})
		return err
	})
	return userID, oldRole, err
}
