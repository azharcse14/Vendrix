// Package migrations: database migration file (6.1, 6.6). Shudhu ei folder er top-level *.sql embed hoy;
// constraint test tests/ e (embed e ashe na, v1.6).
//
// Feature doc: docs/features/migration.md
package migrations

import "embed"

// FS: `shop-api migrate` ei file gulo chalay (internal/db.Migrate).
//
//go:embed *.sql
var FS embed.FS
