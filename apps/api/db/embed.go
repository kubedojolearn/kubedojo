// Package db embeds the SQL migrations so the API binary can migrate without extra files.
package db

import "embed"

// Migrations holds the goose SQL migrations (db/migrations/*.sql).
//
//go:embed migrations/*.sql
var Migrations embed.FS
