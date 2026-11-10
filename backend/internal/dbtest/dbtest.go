// Package dbtest: asol Postgres e test er jonno notun database, embed kora migration (001) shoho (14.1).
// TEST_DATABASE_URL na thakle test skip; database test shesh e muche jay.
//
// Feature doc: docs/features/migration.md
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"vendrix/backend/internal/db"
	"vendrix/backend/migrations"
)

// New: notun C.UTF-8 database, shob migration chalano (app.env local), *sql.DB ferot. TEST_DATABASE_URL e
// CREATEDB onumoti wala user (local: make db-up er postgres user).
func New(t testing.TB) *sql.DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL nai")
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Skip("TEST_DATABASE_URL postgres://... akare dao")
	}
	ctx := context.Background()
	admin, err := db.Open(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := "vendrix_t_" + strings.ToLower(rand.Text()[:10])
	// rule12:ok database er naam ei function er random, bahir theke na
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE `+name+` TEMPLATE template0 ENCODING 'UTF8' LOCALE 'C.UTF-8'`); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	u.Path = "/" + name
	d, err := db.Open(ctx, u.String())
	t.Cleanup(func() {
		if d != nil {
			d.Close()
		}
		admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`) // rule12:ok
		admin.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := db.LoadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(ctx, d, files, "local"); err != nil {
		t.Fatal(err)
	}
	return d
}
