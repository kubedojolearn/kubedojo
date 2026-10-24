package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/kubedojolearn/kubedojo/apps/api/db"
)

// Migrator applies the embedded goose migrations. Migrations are forward-only in production;
// Down is for development and tests (see docs/operations/database.md).
type Migrator struct {
	provider *goose.Provider
	sqlDB    *sql.DB
}

// NewMigrator builds a migrator over pool's connection settings.
func NewMigrator(pool *pgxpool.Pool, log *slog.Logger) (*Migrator, error) {
	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return nil, err
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations,
		goose.WithLogger(gooseLogger{log}),
	)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return &Migrator{provider: p, sqlDB: sqlDB}, nil
}

// Close releases the migrator's database handle (the pool stays open).
func (m *Migrator) Close() error { return m.sqlDB.Close() }

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) ([]*goose.MigrationResult, error) {
	return m.provider.Up(ctx)
}

// DownTo rolls back to version (0 = empty schema). Development and tests only.
func (m *Migrator) DownTo(ctx context.Context, version int64) ([]*goose.MigrationResult, error) {
	return m.provider.DownTo(ctx, version)
}

// Down rolls back the most recent migration. Development and tests only.
func (m *Migrator) Down(ctx context.Context) (*goose.MigrationResult, error) {
	return m.provider.Down(ctx)
}

// Status reports every migration and whether it is applied.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	return m.provider.Status(ctx)
}

// Version returns the current schema version.
func (m *Migrator) Version(ctx context.Context) (int64, error) { return m.provider.GetDBVersion(ctx) }

type gooseLogger struct{ log *slog.Logger }

func (g gooseLogger) Printf(format string, v ...any) { g.log.Info(fmt.Sprintf(format, v...)) }
func (g gooseLogger) Fatalf(format string, v ...any) { g.log.Error(fmt.Sprintf(format, v...)) }
