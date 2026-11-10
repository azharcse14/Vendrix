// Command api: Vendrix er Go API (binary shop-api, 13.4). Subcommand: serve (default), migrate,
// shop create, credentials set, version.
//
// Feature doc: docs/features/server.md (serve), docs/features/migration.md (migrate),
// docs/features/credentials.md (credentials), docs/features/shop.md (shop create)
package main

import (
	"context"
	"errors"
	"flag"
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
	"vendrix/backend/internal/shop"
	"vendrix/backend/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.LookupEnv, os.Stdin, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shop-api:", err)
		os.Exit(1)
	}
}

// run: main er asol kaj; test er jonno env (lookup), stdin ar output bahir theke ashe.
func run(ctx context.Context, args []string, lookup func(string) (string, bool), in io.Reader, out io.Writer) error {
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
	case "credentials":
		return runCredentials(ctx, args[1:], lookup, in, log)
	case "shop":
		return runShopCreate(ctx, args[1:], lookup, out, log)
	case "version":
		_, err := fmt.Fprintln(out, version())
		return err
	}
	return fmt.Errorf("ochena command %q: serve, migrate, shop, credentials ba version", cmd)
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
	h, err := newHandler(cfg, log, database, database)
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

// maxSecret: stdin er secret JSON er shima.
const maxSecret = 64 << 10

// runCredentials (3.8): `credentials set --shop <slug> --provider <p>`, secret JSON stdin theke (shell history
// ba process list e jay na).
func runCredentials(ctx context.Context, args []string, lookup func(string) (string, bool), in io.Reader, log *slog.Logger) error {
	if len(args) == 0 || args[0] != "set" {
		return errors.New("credentials set --shop <slug> --provider <provider> (secret JSON stdin e)")
	}
	fs := flag.NewFlagSet("credentials set", flag.ContinueOnError)
	slug := fs.String("shop", "", "shop er slug")
	provider := fs.String("provider", "", "sslcommerz, bkash, nagad, sms, steadfast, pathao ba redx")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *slug == "" || *provider == "" || fs.NArg() > 0 {
		return errors.New("--shop ar --provider lagbe")
	}
	cfg, err := config.Load(lookup, config.Credentials)
	if err != nil {
		return err
	}
	secret, err := io.ReadAll(io.LimitReader(in, maxSecret+1))
	if err != nil {
		return err
	}
	if len(secret) > maxSecret {
		return fmt.Errorf("secret %d KB er beshi", maxSecret>>10)
	}
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := shop.SetCredential(ctx, database, cfg.CredentialsKey, *slug, *provider, secret); err != nil {
		return err
	}
	log.Info("credential save", "shop", *slug, "provider", *provider)
	return nil
}

// runShopCreate (3.8, v1.10): `shop create --slug --prefix --domain --name-bn [--name-en] --owner-email
// --owner-name [--preset organic]`. Owner er temporary password shudhu out e ekbar, log e na.
func runShopCreate(ctx context.Context, args []string, lookup func(string) (string, bool), out io.Writer, log *slog.Logger) error {
	usage := errors.New("shop create --slug <slug> --prefix <SB> --domain <domain> --name-bn <naam> [--name-en <name>] --owner-email <email> --owner-name <naam> [--preset organic]")
	if len(args) == 0 || args[0] != "create" {
		return usage
	}
	var p shop.Params
	fs := flag.NewFlagSet("shop create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&p.Slug, "slug", "", "")
	fs.StringVar(&p.Prefix, "prefix", "", "")
	fs.StringVar(&p.Domain, "domain", "", "")
	fs.StringVar(&p.NameBn, "name-bn", "", "")
	fs.StringVar(&p.NameEn, "name-en", "", "")
	fs.StringVar(&p.OwnerEmail, "owner-email", "", "")
	fs.StringVar(&p.OwnerName, "owner-name", "", "")
	fs.StringVar(&p.Preset, "preset", "organic", "")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 || p.Slug == "" || p.Prefix == "" || p.Domain == "" ||
		p.NameBn == "" || p.OwnerEmail == "" || p.OwnerName == "" {
		return usage
	}
	cfg, err := config.Load(lookup, config.Shop)
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	id, password, err := shop.Create(ctx, database, p)
	if err != nil {
		return err
	}
	log.Info("shop create", "shop_id", id, "slug", p.Slug, "domain", p.Domain)
	_, err = fmt.Fprintf(out, "Owner %s er temporary password (ekbar-i dekhano hocche; prothom login e bodlate hobe): %s\n", p.OwnerEmail, password)
	return err
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
