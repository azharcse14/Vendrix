package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestMigrateIntegration (6.1, 14.2 test 42): asol Postgres e notun database banay, Migrate duibar (ditiyo
// bar 0 ta), set_config('app.env'), applied file bodlale ba out-of-order file e fail, shesh e database
// muche fele. TEST_DATABASE_URL e CREATEDB onumoti wala user (local: make db-up er postgres user).
// pgx driver na thakle (M0 er driver step er age) skip.
func TestMigrateIntegration(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL nai")
	}
	if !slices.Contains(sql.Drivers(), DriverName) {
		t.Skip("pgx driver nai (docs/milestones/M0.md)")
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Skip("TEST_DATABASE_URL postgres://... akare dao")
	}
	ctx := context.Background()
	admin, err := Open(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "vendrix_mig_" + strings.ToLower(rand.Text()[:10])
	// rule12:ok database er naam ei test er random, bahir theke na
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE `+name+` TEMPLATE template0 ENCODING 'UTF8' LOCALE 'C.UTF-8'`); err != nil {
		t.Fatal(err)
	}
	defer admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`) // rule12:ok
	u.Path = "/" + name
	d, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	mk := func(version, q string) Migration {
		sum := sha256.Sum256([]byte(q))
		return Migration{Version: version, SQL: q, SHA256: hex.EncodeToString(sum[:])}
	}
	files := []Migration{
		mk("001_a", "CREATE TABLE a AS SELECT current_setting('app.env') AS env; CREATE TABLE a2 (id int);"),
		mk("002_b", "CREATE TABLE b (id int);"),
	}
	if n, err := Migrate(ctx, d, files, "local"); err != nil || n != 2 {
		t.Fatalf("prothom bar: %d %v", n, err)
	}
	if n, err := Migrate(ctx, d, files, "local"); err != nil || n != 0 {
		t.Fatalf("ditiyo bar: %d %v", n, err)
	}
	var env string
	if err := d.QueryRowContext(ctx, `SELECT env FROM a`).Scan(&env); err != nil || env != "local" {
		t.Errorf("app.env %q %v", env, err)
	}
	changed := slices.Clone(files)
	changed[0] = mk("001_a", files[0].SQL+" -- bodla")
	if _, err := Migrate(ctx, d, changed, "local"); err == nil || !strings.Contains(err.Error(), "bodleche") {
		t.Errorf("checksum: %v", err)
	}
	late := append(slices.Clone(files), mk("001_z", "SELECT 1"))
	if _, err := Migrate(ctx, d, late, "local"); err == nil || !strings.Contains(err.Error(), "choto number") {
		t.Errorf("out-of-order: %v", err)
	}
}
