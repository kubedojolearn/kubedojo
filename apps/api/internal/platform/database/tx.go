package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQL error codes the data layer inspects.
const (
	CodeUniqueViolation      = "23505"
	CodeForeignKeyViolation  = "23503"
	CodeRestrictViolation    = "23001"
	CodeCheckViolation       = "23514"
	CodeIntegrityViolation   = "23000"
	codeSerializationFailure = "40001"
	codeDeadlockDetected     = "40P01"
)

const maxTxAttempts = 3

// IsCode reports whether err is a PostgreSQL error with the given SQLSTATE.
func IsCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

// ConstraintName returns the violated constraint, if any.
func ConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

// WithTx runs fn in a transaction: commit on nil, rollback on error or panic. Serialization
// failures and deadlocks are retried (fn must therefore be safe to re-run from the start).
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	var err error
	for attempt := 1; attempt <= maxTxAttempts; attempt++ {
		err = pgx.BeginFunc(ctx, pool, fn)
		if err == nil || (!IsCode(err, codeSerializationFailure) && !IsCode(err, codeDeadlockDetected)) {
			return err
		}
	}
	return fmt.Errorf("transaction failed after %d attempts: %w", maxTxAttempts, err)
}
