package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/dbtest"
)

// TestBackoff (8.17): fail er por 1, 5, 15, 60 minute, tarpor 60 e thake.
func TestBackoff(t *testing.T) {
	want := map[int]time.Duration{1: time.Minute, 2: 5 * time.Minute, 3: 15 * time.Minute, 4: time.Hour, 9: time.Hour}
	for attempts, d := range want {
		if got := backoff(attempts); got != d {
			t.Errorf("backoff(%d) = %v, want %v", attempts, got, d)
		}
	}
}

type jobRow struct {
	attempts          int
	done, failed      bool
	locked            bool
	lastError         string
	runAtAfterMinutes float64 // run_at - now(), minute e
}

func get(t *testing.T, d *sql.DB, kind string) jobRow {
	t.Helper()
	var r jobRow
	var le sql.NullString
	err := d.QueryRow(`SELECT attempts, done_at IS NOT NULL, failed_at IS NOT NULL, locked_at IS NOT NULL, last_error,
		extract(epoch FROM run_at - now()) / 60 FROM jobs WHERE kind = $1`, kind).
		Scan(&r.attempts, &r.done, &r.failed, &r.locked, &le, &r.runAtAfterMinutes)
	if err != nil {
		t.Fatalf("%s: %v", kind, err)
	}
	r.lastError = le.String
	return r
}

// TestWorkerIntegration (6.4, 8.17): asol Postgres e enqueue (dedupe), claim, shofol, fail e backoff,
// max_attempts e failed_at ar alert, panic, onno worker lock nile purono worker er result bad, jobs.sweep.
func TestWorkerIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	var logs bytes.Buffer
	w := NewWorker(d, slog.New(slog.NewTextHandler(&logs, nil)), map[string]Handler{
		"t.ok":    func(context.Context, Job) error { return nil },
		"t.bad":   func(context.Context, Job) error { return errors.New("gateway down") },
		"t.panic": func(context.Context, Job) error { panic("boom") },
		"t.stolen": func(ctx context.Context, j Job) error {
			// onno worker 5 minute por lock niyeche
			_, err := d.ExecContext(ctx, `UPDATE jobs SET locked_at = now() + interval '1 second' WHERE id = $1`, j.ID)
			return err
		},
	})

	for _, n := range []New{
		{Kind: "t.ok", Payload: map[string]string{"a": "b"}, DedupeKey: "t.ok:1"},
		{Kind: "t.ok", DedupeKey: "t.ok:1"}, // ekoi key: notun row na
		{Kind: "t.bad", MaxAttempts: 2},
		{Kind: "t.panic"},
		{Kind: "t.stolen"},
		{Kind: "t.later", RunAt: time.Now().Add(time.Hour)},
	} {
		if err := Enqueue(ctx, d, n); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := d.QueryRow(`SELECT count(*) FROM jobs WHERE kind = 't.ok'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dedupe: %d %v", count, err)
	}

	if n, err := w.RunOnce(ctx); err != nil || n != 4 {
		t.Fatalf("RunOnce: %d %v (t.later er shomoy hoyni)", n, err)
	}
	if r := get(t, d, "t.ok"); !r.done || r.locked || r.attempts != 1 {
		t.Errorf("t.ok: %+v", r)
	}
	if r := get(t, d, "t.bad"); r.done || r.failed || r.locked || r.attempts != 1 ||
		r.lastError != "gateway down" || r.runAtAfterMinutes < 0.9 || r.runAtAfterMinutes > 1.1 {
		t.Errorf("t.bad prothom fail (1 minute backoff): %+v", r)
	}
	if r := get(t, d, "t.panic"); r.done || !strings.Contains(r.lastError, "boom") {
		t.Errorf("t.panic: %+v", r)
	}
	if r := get(t, d, "t.stolen"); r.done || !r.locked {
		t.Errorf("t.stolen: purono worker er result lekha hoyeche: %+v", r)
	}
	if r := get(t, d, "t.later"); r.attempts != 0 {
		t.Errorf("t.later: %+v", r)
	}

	// shesh attempt: failed_at ar admin alert
	if _, err := d.Exec(`UPDATE jobs SET run_at = now() WHERE kind = 't.bad'`); err != nil {
		t.Fatal(err)
	}
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if r := get(t, d, "t.bad"); !r.failed || r.attempts != 2 || r.locked {
		t.Errorf("t.bad shesh attempt: %+v", r)
	}
	if !strings.Contains(logs.String(), "admin alert") || !strings.Contains(logs.String(), "t.bad") {
		t.Errorf("alert log nai: %s", logs.String())
	}

	// shesh attempt e crash: jobs.sweep failed kore
	if err := Enqueue(ctx, d, New{Kind: "t.crash", MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`UPDATE jobs SET attempts = 1, locked_at = now() - interval '6 minutes' WHERE kind = 't.crash'`); err != nil {
		t.Fatal(err)
	}
	if err := w.sweep(ctx, Job{}); err != nil {
		t.Fatal(err)
	}
	if r := get(t, d, "t.crash"); !r.failed || r.locked || r.lastError != "worker crash (lock timeout)" {
		t.Errorf("t.crash: %+v", r)
	}
}

// TestScheduleSweep (8.17): jobs.sweep protiti 5 minute er slot e ekbar-i.
func TestScheduleSweep(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := NewWorker(d, slog.New(slog.DiscardHandler), nil)
	t0 := time.Date(2026, 11, 10, 12, 1, 0, 0, time.UTC)
	for _, at := range []time.Time{t0, t0.Add(2 * time.Minute), t0.Add(4 * time.Minute)} {
		if err := w.schedule(ctx, at); err != nil {
			t.Fatal(err)
		}
	}
	var keys []string
	rows, err := d.Query(`SELECT dedupe_key FROM jobs WHERE kind = 'jobs.sweep' ORDER BY dedupe_key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	want := "jobs.sweep:2026-11-10T12:00:00Z jobs.sweep:2026-11-10T12:05:00Z"
	if got := strings.Join(keys, " "); got != want {
		t.Errorf("keys = %q, want %q", got, want)
	}
}
