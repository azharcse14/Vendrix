// Package jobs: background kaj er queue (8.17): jobs table e Enqueue, Worker 6.4 er query diye kaj nay
// (FOR UPDATE SKIP LOCKED), fail e backoff, max_attempts e failed_at ar admin alert, jobs.sweep.
//
// Feature doc: docs/features/jobs.md
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Job: worker handler ke ja dey. ShopID 0 = platform er kaj (shop_id NULL).
type Job struct {
	ID      int64
	ShopID  int64
	Kind    string
	Payload json.RawMessage
}

// Handler: ek kind er kaj. Idempotent hote hobe (ekoi job abar chalte pare, 8.17). Error dile retry.
type Handler func(ctx context.Context, j Job) error

// New: notun job. RunAt khali = database er now() (app ar DB er ghori na milleo shathe shathe chole); MaxAttempts 0 = 5; DedupeKey thakle ekoi key e duibar insert hoy na.
type New struct {
	ShopID      int64
	Kind        string
	Payload     any
	DedupeKey   string
	RunAt       time.Time
	MaxAttempts int
}

// Execer: *sql.DB ba *sql.Tx; order er transaction er bhetore enqueue korle commit holei job ashe.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Enqueue: jobs e insert; DedupeKey er conflict e kichu kore na.
func Enqueue(ctx context.Context, e Execer, n New) error {
	payload := []byte("{}")
	if n.Payload != nil {
		var err error
		if payload, err = json.Marshal(n.Payload); err != nil {
			return fmt.Errorf("job %s payload: %w", n.Kind, err)
		}
	}
	if n.MaxAttempts == 0 {
		n.MaxAttempts = 5
	}
	_, err := e.ExecContext(ctx, `INSERT INTO jobs (shop_id, kind, payload, dedupe_key, run_at, max_attempts)
		VALUES (NULLIF($1, 0), $2, $3, NULLIF($4, ''), coalesce($5, now()), $6)
		ON CONFLICT (dedupe_key) DO NOTHING`,
		n.ShopID, n.Kind, payload, n.DedupeKey, sql.NullTime{Time: n.RunAt, Valid: !n.RunAt.IsZero()}, n.MaxAttempts)
	return err
}

// backoff (8.17): attempts (ei bar shoho) er por porer chesta kokhon.
func backoff(attempts int) time.Duration {
	switch attempts {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 15 * time.Minute
	}
	return time.Hour
}

const (
	batchSize     = 10
	pollInterval  = 2 * time.Second
	sweepInterval = 5 * time.Minute
	// handlerTimeout: ek job er shima; lock er 5 minute er onek kom.
	handlerTimeout = time.Minute
)

// Worker: ek goroutine e kaj nay ar chalay. Onek process e chalaleo ek kaj ek jon nay (SKIP LOCKED).
type Worker struct {
	db        *sql.DB
	log       *slog.Logger
	handlers  map[string]Handler
	lastSweep time.Time
}

// NewWorker: handlers kind → Handler; jobs.sweep nijei jog hoy.
func NewWorker(d *sql.DB, log *slog.Logger, handlers map[string]Handler) *Worker {
	w := &Worker{db: d, log: log, handlers: map[string]Handler{}}
	for k, h := range handlers {
		w.handlers[k] = h
	}
	w.handlers["jobs.sweep"] = w.sweep
	return w
}

// Run: ctx bondho na hoya porjonto chole (serve er shutdown e thame). Database er error log kore abar chesta.
func (w *Worker) Run(ctx context.Context) {
	for {
		n, err := 0, w.schedule(ctx, time.Now())
		if err == nil {
			n, err = w.RunOnce(ctx)
		}
		if err != nil && ctx.Err() == nil {
			w.log.Error("job worker", "err", err)
		}
		if n == batchSize {
			continue // aro kaj thakte pare
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// schedule: recurring job, slot er dedupe_key diye (duita process e-o ekbar).
func (w *Worker) schedule(ctx context.Context, now time.Time) error {
	slot := now.UTC().Truncate(sweepInterval)
	if slot.Equal(w.lastSweep) {
		return nil
	}
	err := Enqueue(ctx, w.db, New{Kind: "jobs.sweep", DedupeKey: "jobs.sweep:" + slot.Format(time.RFC3339), RunAt: slot})
	if err == nil {
		w.lastSweep = slot
	}
	return err
}

// RunOnce: ekbar claim (6.4, batchSize porjonto) ar protiti kaj chalay; koyta niyeche ferot dey.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	rows, err := w.db.QueryContext(ctx, `WITH next AS (
		  SELECT id FROM jobs
		  WHERE done_at IS NULL AND failed_at IS NULL AND attempts < max_attempts AND run_at <= now()
		    AND (locked_at IS NULL OR locked_at < now() - interval '5 minutes')
		  ORDER BY run_at
		  FOR UPDATE SKIP LOCKED
		  LIMIT $1)
		UPDATE jobs j SET locked_at = now(), attempts = attempts + 1
		FROM next WHERE j.id = next.id
		RETURNING j.id, coalesce(j.shop_id, 0), j.kind, j.payload, j.locked_at, j.attempts`, batchSize)
	if err != nil {
		return 0, err
	}
	var todo []claimed
	for rows.Next() {
		var c claimed
		if err := rows.Scan(&c.ID, &c.ShopID, &c.Kind, &c.Payload, &c.lockedAt, &c.attempts); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, c)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return 0, err
	}
	for i, c := range todo {
		// shutdown e baki job na chaliye chere dao, na hole bondho ctx e fail hoye attempt khoroch
		if err := ctx.Err(); err != nil {
			return len(todo), errors.Join(err, w.release(todo[i:]))
		}
		// batch er ager job dhire cholle 5 minute por onno worker eta nite pare; nile bad, na hole SMS duibar
		err := w.db.QueryRowContext(ctx, `UPDATE jobs SET locked_at = now() WHERE id = $1 AND locked_at = $2
			RETURNING locked_at`, c.ID, c.lockedAt).Scan(&c.lockedAt)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return len(todo), errors.Join(err, w.release(todo[i:]))
		}
		hctx, cancel := context.WithTimeout(ctx, handlerTimeout)
		jobErr := w.call(hctx, c.Job)
		cancel()
		if err := w.finish(c.Job, c.lockedAt, c.attempts, jobErr); err != nil {
			return len(todo), errors.Join(err, w.release(todo[i+1:]))
		}
	}
	return len(todo), nil
}

type claimed struct {
	Job
	lockedAt time.Time
	attempts int
}

// release: claim kora kintu na chala job ager obosthay (attempt ferot), jate shathe shathe abar neya jay.
func (w *Worker) release(rest []claimed) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var errs []error
	for _, c := range rest {
		_, err := w.db.ExecContext(ctx, `UPDATE jobs SET locked_at = NULL, attempts = attempts - 1
			WHERE id = $1 AND locked_at = $2`, c.ID, c.lockedAt)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (w *Worker) call(ctx context.Context, j Job) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	h, ok := w.handlers[j.Kind]
	if !ok {
		return fmt.Errorf("ochena job kind %q", j.Kind)
	}
	return h(ctx, j)
}

// finish: result lekha. locked_at mile na hole (5 minute por onno worker niyeche) kichu lekhe na.
// Shutdown e ctx bondho holeo result jate lekha hoy, tai nijer context.
func (w *Worker) finish(j Job, lockedAt time.Time, attempts int, jobErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if jobErr == nil {
		_, err := w.db.ExecContext(ctx, `UPDATE jobs SET done_at = now(), locked_at = NULL, last_error = NULL
			WHERE id = $1 AND locked_at = $2`, j.ID, lockedAt)
		return err
	}
	var failed bool
	err := w.db.QueryRowContext(ctx, `UPDATE jobs SET locked_at = NULL, last_error = $3,
		  run_at = now() + make_interval(secs => $4),
		  failed_at = CASE WHEN attempts >= max_attempts THEN now() END
		WHERE id = $1 AND locked_at = $2
		RETURNING failed_at IS NOT NULL`,
		j.ID, lockedAt, jobErr.Error(), backoff(attempts).Seconds()).Scan(&failed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if failed {
		w.alert(j.ID, j.ShopID, j.Kind, jobErr.Error())
	}
	return nil
}

// sweep (jobs.sweep, 6.4): shesh attempt e crash kora job failed ar alert.
func (w *Worker) sweep(ctx context.Context, _ Job) error {
	rows, err := w.db.QueryContext(ctx, `UPDATE jobs SET failed_at = now(), locked_at = NULL,
		  last_error = coalesce(last_error, 'worker crash (lock timeout)')
		WHERE done_at IS NULL AND failed_at IS NULL AND attempts >= max_attempts
		  AND locked_at < now() - interval '5 minutes'
		RETURNING id, coalesce(shop_id, 0), kind, last_error`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, shopID int64
		var kind, lastErr string
		if err := rows.Scan(&id, &shopID, &kind, &lastErr); err != nil {
			return err
		}
		w.alert(id, shopID, kind, lastErr)
	}
	return rows.Err()
}

// ponytail: admin alert ekhon shudhu error log; ALERT_EMAIL/ALERT_WEBHOOK_URL (13.5) M12 e.
func (w *Worker) alert(id, shopID int64, kind, lastErr string) {
	w.log.Error("admin alert: job failed", "job_id", id, "shop_id", shopID, "kind", kind, "err", lastErr)
}
