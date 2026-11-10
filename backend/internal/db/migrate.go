package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

// Migration: ekta migration file (6.1). Version = file er naam .sql chhara ("001_init").
type Migration struct {
	Version string
	SQL     string
	SHA256  string // hex; schema_migrations.sha256 (v1.7)
}

var migrationName = regexp.MustCompile(`^(\d{3})_[a-z0-9_]+\.sql$`)

// LoadMigrations: fsys er top-level *.sql, naam onujayi (backend/migrations.FS; tests/ embed e ashe na).
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	seen := map[string]string{}
	var out []Migration
	for _, n := range names {
		m := migrationName.FindStringSubmatch(n)
		if m == nil || strings.HasSuffix(n, "_test.sql") {
			return nil, fmt.Errorf("migration %q: naam 00N_choto_hater_naam.sql, test file tests/ e (6.1)", n)
		}
		if prev, ok := seen[m[1]]; ok {
			return nil, fmt.Errorf("migration %q ar %q ekoi number", prev, n)
		}
		seen[m[1]] = n
		b, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		out = append(out, Migration{Version: strings.TrimSuffix(n, ".sql"), SQL: string(b), SHA256: hex.EncodeToString(sum[:])})
	}
	if len(out) == 0 {
		return nil, errors.New("kono migration file nai")
	}
	return out, nil
}

// Pending (pure, 6.1 v1.7): database e applied (version → sha256) er sathe file milay. Applied file er hash
// na mille (deploy er por bodleche), applied file harale, ba shesh applied er cheye choto number er notun
// file thakle error: tokhon kono migration chole na.
func Pending(files []Migration, applied map[string]string) ([]Migration, error) {
	byVersion := make(map[string]Migration, len(files))
	for _, f := range files {
		byVersion[f.Version] = f
	}
	versions := make([]string, 0, len(applied))
	for v := range applied {
		versions = append(versions, v)
	}
	slices.Sort(versions)
	last := ""
	for _, v := range versions {
		f, ok := byVersion[v]
		switch {
		case !ok:
			return nil, fmt.Errorf("migration %s database e applied, kintu file nai (naam bodleche ba muche geche?)", v)
		case f.SHA256 != applied[v]:
			return nil, fmt.Errorf("migration %s deploy er por bodleche (sha256 mile na): ager moto koro, bodol notun 00N_*.sql e (6.1)", v)
		}
		last = v
	}
	var pending []Migration
	for _, f := range files {
		if _, ok := applied[f.Version]; ok {
			continue
		}
		if f.Version < last {
			return nil, fmt.Errorf("migration %s shesh applied %s er cheye choto number: porer number dao (6.6)", f.Version, last)
		}
		pending = append(pending, f)
	}
	return pending, nil
}

// lockKey: migration runner er pg_advisory_xact_lock (duita runner ek shathe cholle ekta opekkha kore).
const lockKey int64 = 7_160_031_001

const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
  version    TEXT PRIMARY KEY,
  sha256     TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Migrate (6.1): lock er bhetore schema_migrations banay, Pending dekhe (checksum, out-of-order), tarpor
// protiti notun file nijer transaction e: lock → schema_migrations abar dekha (onno runner chaliye thakle
// skip) → set_config('app.env', $1, true) → file → insert. Ferot dey koyta file chollo.
func Migrate(ctx context.Context, d *sql.DB, files []Migration, appEnv string) (int, error) {
	err := InTx(ctx, d, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, createTable)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("schema_migrations: %w", err)
	}
	applied, err := appliedVersions(ctx, d)
	if err != nil {
		return 0, err
	}
	pending, err := Pending(files, applied)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range pending {
		ran := false
		err := InTx(ctx, d, func(tx *sql.Tx) error {
			ran = false
			if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
				return err
			}
			var sum string
			err := tx.QueryRowContext(ctx, `SELECT sha256 FROM schema_migrations WHERE version = $1`, m.Version).Scan(&sum)
			switch {
			case err == nil && sum == m.SHA256:
				return nil // onno runner ei matro chaliyeche
			case err == nil:
				return errors.New("onno runner bhinno sha256 diye chaliyeche")
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.env', $1, true)`, appEnv); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, sha256) VALUES ($1, $2)`, m.Version, m.SHA256); err != nil {
				return err
			}
			ran = true
			return nil
		})
		if err != nil {
			return n, fmt.Errorf("migration %s: %w", m.Version, err)
		}
		if ran {
			n++
		}
	}
	return n, nil
}

func appliedVersions(ctx context.Context, d *sql.DB) (map[string]string, error) {
	rows, err := d.QueryContext(ctx, `SELECT version, sha256 FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var v, s string
		if err := rows.Scan(&v, &s); err != nil {
			return nil, err
		}
		out[v] = s
	}
	return out, rows.Err()
}
