// Command api: Vendrix er Go API (binary shop-api, 13.4). Subcommand: serve (default), migrate, version.
//
// Feature doc: docs/features/server.md (serve), docs/features/migration.md (migrate)
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"vendrix/backend/internal/config"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/notify"
	"vendrix/backend/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shop-api:", err)
		os.Exit(1)
	}
}

// run: main er asol kaj; test er jonno env (lookup) ar output bahir theke ashe.
func run(ctx context.Context, args []string, lookup func(string) (string, bool), out io.Writer) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	log := slog.New(slog.NewJSONHandler(out, nil))
	switch cmd {
	case "serve":
		return runServe(ctx, lookup, log)
	case "migrate":
		return runMigrate(ctx, lookup, log)
	case "version":
		_, err := fmt.Fprintln(out, version())
		return err
	}
	return fmt.Errorf("ochena command %q: serve, migrate ba version", cmd)
}

func runServe(ctx context.Context, lookup func(string) (string, bool), log *slog.Logger) error {
	cfg, err := config.Load(lookup, config.Serve)
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	h, err := newHandler(cfg, log, database)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	w := jobs.NewWorker(database, log, map[string]jobs.Handler{
		"sms.send": notify.SendSMSJob(notify.NewSMS(cfg.Env, log)),
	})
	wctx, stopWorker := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { w.Run(wctx); close(done) }()
	log.Info("server shuru", "addr", cfg.Addr, "env", cfg.Env, "version", version())
	err = serve(ctx, newServer(h), ln, log)
	// cholche emon job er result lekha porjonto apekkha (database close er age)
	stopWorker()
	<-done
	return err
}

func runMigrate(ctx context.Context, lookup func(string) (string, bool), log *slog.Logger) error {
	cfg, err := config.Load(lookup, config.Migrate)
	if err != nil {
		return err
	}
	files, err := db.LoadMigrations(migrations.FS)
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, cfg.MigrateDatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	n, err := db.Migrate(ctx, database, files, cfg.Env)
	if err != nil {
		return err
	}
	log.Info("migration shesh", "notun", n, "mot", len(files))
	return nil
}

// version: build er git commit (go build VCS info), na thakle "dev".
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "dev"
}
