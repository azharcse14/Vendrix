package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
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

// TestBatchStolenLockIntegration (8.17): batch er porer job er lock onno worker nile purono worker oita
// chalay na (SMS duibar na); handler er ctx e deadline thake.
func TestBatchStolenLockIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	ran := 0
	var deadline time.Duration
	// duita job ek batch e; je age chole se onnotar lock "onno worker" er hate dey (handler dhire cholchilo)
	steal := func(other string) Handler {
		return func(ctx context.Context, j Job) error {
			ran++
			if dl, ok := ctx.Deadline(); ok {
				deadline = time.Until(dl)
			}
			_, err := d.ExecContext(ctx, `UPDATE jobs SET locked_at = now() + interval '1 second' WHERE kind = $1`, other)
			return err
		}
	}
	w := NewWorker(d, slog.New(slog.DiscardHandler), map[string]Handler{"t.a": steal("t.b"), "t.b": steal("t.a")})
	for _, n := range []New{{Kind: "t.a"}, {Kind: "t.b"}} {
		if err := Enqueue(ctx, d, n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if ran != 1 {
		t.Errorf("%d ta job cholche, chai 1 (churi kora lock er job cholbe na)", ran)
	}
	if deadline <= 0 || deadline > handlerTimeout {
		t.Errorf("handler er deadline %v, chai 0 < x <= %v", deadline, handlerTimeout)
	}
}

// TestConcurrentClaimIntegration (6.4): duita worker ek shathe nile protiti job ekbar-i chole; 5 minute er
// purono lock abar neya jay.
func TestConcurrentClaimIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	var mu sync.Mutex
	seen := map[int64]int{}
	h := map[string]Handler{"t.n": func(_ context.Context, j Job) error {
		mu.Lock()
		seen[j.ID]++
		mu.Unlock()
		return nil
	}}
	for range 30 {
		if err := Enqueue(ctx, d, New{Kind: "t.n"}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for range 2 {
		w := NewWorker(d, slog.New(slog.DiscardHandler), h)
		wg.Go(func() {
			for {
				n, err := w.RunOnce(ctx)
				if err != nil {
					t.Error(err)
				}
				if n == 0 || err != nil {
					return
				}
			}
		})
	}
	wg.Wait()
	if len(seen) != 30 {
		t.Errorf("%d ta job cholche, chai 30", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("job %d cholche %d bar", id, n)
		}
	}

	if _, err := d.Exec(`INSERT INTO jobs (kind, attempts, locked_at) VALUES ('t.n', 1, now() - interval '6 minutes')`); err != nil {
		t.Fatal(err)
	}
	if n, err := NewWorker(d, slog.New(slog.DiscardHandler), h).RunOnce(ctx); err != nil || n != 1 {
		t.Errorf("5 minute er purono lock: %d %v", n, err)
	}
}

// TestShutdownReleasesIntegration: ctx bondho hole batch er baki job chalay na, attempt o khoroch hoy na.
func TestShutdownReleasesIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	ran := 0
	w := NewWorker(d, slog.New(slog.DiscardHandler), map[string]Handler{"t.s": func(context.Context, Job) error {
		ran++
		cancel() // prothom job er majhe shutdown
		return nil
	}})
	for range 3 {
		if err := Enqueue(context.Background(), d, New{Kind: "t.s"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var waiting int
	if err := d.QueryRow(`SELECT count(*) FROM jobs WHERE kind = 't.s' AND done_at IS NULL AND attempts = 0 AND locked_at IS NULL`).Scan(&waiting); err != nil {
		t.Fatal(err)
	}
	if ran != 1 || waiting != 2 {
		t.Errorf("chole %d (chai 1), ager moto baki %d (chai 2)", ran, waiting)
	}
}
