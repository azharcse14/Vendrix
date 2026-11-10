// Package db: Postgres connection (database/sql + pgx, 2.1), transaction helper (8.6 er retry) ar
// migration runner (6.1).
//
// Feature doc: docs/features/migration.md
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
)

// DriverName: pgx/v5/stdlib er database/sql driver. M0: internal/db/driver.go e
// `import _ "github.com/jackc/pgx/v5/stdlib"` (docs/milestones/M0.md); tar age Open spashto error dey.
const DriverName = "pgx"

// ErrNoDriver: pgx driver register hoyni.
var ErrNoDriver = errors.New(`database driver "pgx" register hoyni: docs/milestones/M0.md er "pgx driver" step koro`)

// Open: pool banay, tarpor 5 second er moddhe ping.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	if !slices.Contains(sql.Drivers(), DriverName) {
		return nil, ErrNoDriver
	}
	d, err := sql.Open(DriverName, url)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(20)
	d.SetMaxIdleConns(10)
	d.SetConnMaxIdleTime(5 * time.Minute)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.PingContext(pctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("database e jete parchi na: %w", err)
	}
	return d, nil
}

// InTx: fn ek transaction e (READ COMMITTED, 6.4). fn error dile ba panic korle ROLLBACK. Deadlock
// (40P01) ba serialization (40001) hole puro transaction ekbar abar (8.6), tai fn er shob kaj transaction
// er bhetore rakho (HTTP call ba SMS na; egulo job e, 8.17).
func InTx(ctx context.Context, d *sql.DB, fn func(*sql.Tx) error) error {
	err := inTx(ctx, d, fn)
	if retryable(err) {
		err = inTx(ctx, d, fn)
	}
	return err
}

func inTx(ctx context.Context, d *sql.DB, fn func(*sql.Tx) error) (err error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// retryable: pgconn.PgError er SQLState (pgx import chhara, interface diye dhora).
func retryable(err error) bool {
	var s interface{ SQLState() string }
	if errors.As(err, &s) {
		switch s.SQLState() {
		case "40P01", "40001":
			return true
		}
	}
	return false
}
