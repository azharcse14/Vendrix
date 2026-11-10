// Package config: platform er config env theke (13.5), shuru tei validate: bhul thakle process start-i
// hoy na. Shop er kichu env e na (Rule 15): domain, credential, theme database e.
//
// Feature doc: docs/features/server.md
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// Command: kon subcommand er config (protitir dorkari env alada).
type Command string

const (
	Serve       Command = "serve"
	Migrate     Command = "migrate"
	Credentials Command = "credentials"
	Shop        Command = "shop"
)

// MinSecretLen: secret er kom-poksha dairgho (okkhor). `openssl rand -base64 32` 44 okkhor dey.
const MinSecretLen = 32

// Config: M0 porjonto env (13.5). Notun env lagle: ekhane field ar Load e validation, backend/env.example,
// 13.5 er table, config_test.go te test (docs/features/server.md).
type Config struct {
	Env                 string   // APP_ENV: local, staging, production
	Addr                string   // ADDR: shudhu loopback (v1.5); default 127.0.0.1:8080
	DatabaseURL         string   // DATABASE_URL: shop_app role (serve)
	MigrateDatabaseURL  string   // MIGRATE_DATABASE_URL: table er malik (migrate)
	AdminOrigins        []string // ADMIN_ORIGINS: comma diye (CrossOriginProtection, CORS)
	JasprInternalSecret string   // JASPR_INTERNAL_SECRET: Jaspr → API er X-Internal-Auth (3.2)
	CredentialsKey      []byte   // CREDENTIALS_KEY: base64, 32 byte; shop_credentials er AES-GCM (2.2)
}

// Load: lookup (os.LookupEnv) theke config. Shob bhul ek shathe ferot dey, jate ek bare thik kora jay.
// Error e kokhono secret ba password thake na.
func Load(lookup func(string) (string, bool), cmd Command) (Config, error) {
	get := func(k string) string {
		v, _ := lookup(k)
		return strings.TrimSpace(v)
	}
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	c := Config{Env: get("APP_ENV")}
	switch c.Env {
	case "local", "staging", "production":
	case "":
		bad("APP_ENV nai (local, staging ba production)")
	default:
		bad("APP_ENV %q ochena (local, staging ba production)", c.Env)
	}

	switch cmd {
	case Serve:
		c.Addr = get("ADDR")
		if c.Addr == "" {
			c.Addr = "127.0.0.1:8080"
		}
		if err := loopback(c.Addr); err != nil {
			bad("ADDR: %v", err)
		}
		c.DatabaseURL = get("DATABASE_URL")
		if err := postgresURL(c.DatabaseURL); err != nil {
			bad("DATABASE_URL: %v", err)
		}
		c.JasprInternalSecret = get("JASPR_INTERNAL_SECRET")
		if len(c.JasprInternalSecret) < MinSecretLen {
			bad("JASPR_INTERNAL_SECRET kom-poksha %d okkhor (openssl rand -base64 32)", MinSecretLen)
		}
		for _, o := range strings.Split(get("ADMIN_ORIGINS"), ",") {
			if o = strings.TrimSpace(o); o == "" {
				continue
			}
			if err := origin(o, c.Env); err != nil {
				bad("ADMIN_ORIGINS %q: %v", o, err)
				continue
			}
			c.AdminOrigins = append(c.AdminOrigins, o)
		}
		if len(c.AdminOrigins) == 0 && c.Env != "local" {
			bad("ADMIN_ORIGINS nai (jemon https://admin.tomarshop.com)")
		}
	case Migrate:
		c.MigrateDatabaseURL = get("MIGRATE_DATABASE_URL")
		if err := postgresURL(c.MigrateDatabaseURL); err != nil {
			bad("MIGRATE_DATABASE_URL: %v", err)
		}
	case Credentials, Shop:
		c.DatabaseURL = get("DATABASE_URL")
		if err := postgresURL(c.DatabaseURL); err != nil {
			bad("DATABASE_URL: %v", err)
		}
		if cmd == Credentials {
			k, err := base64.StdEncoding.DecodeString(get("CREDENTIALS_KEY"))
			if err != nil || len(k) != 32 {
				bad("CREDENTIALS_KEY base64 e 32 byte dao (openssl rand -base64 32)")
			}
			c.CredentialsKey = k
		}
	default:
		bad("ochena command %q", cmd)
	}
	// env.example er secret e "local-only" ache; copy kore deploy korle repo-r public secret byabohar hoto
	for name, v := range map[string]string{"JASPR_INTERNAL_SECRET": c.JasprInternalSecret, "CREDENTIALS_KEY": string(c.CredentialsKey)} {
		if c.Env != "local" && strings.Contains(v, "local-only") {
			bad("%s env.example er local-only value; notun dao (openssl rand -base64 32)", name)
		}
	}
	return c, errors.Join(errs...)
}

// loopback: API shudhu loopback e shune, Caddy samne (v1.5, 13.5). ":8080" dile firewall bhul hole API
// Cloudflare ar Caddy chhara-i khola thake.
func loopback(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return fmt.Errorf("%q host:port akare dao", addr)
	}
	if host == "localhost" {
		return nil
	}
	if ip, err := netip.ParseAddr(host); err != nil || !ip.IsLoopback() {
		return fmt.Errorf("%q loopback na (127.0.0.1:8080 dao, Caddy samne thake)", addr)
	}
	return nil
}

// postgresURL: URL er akar dekhe. URL ta nije error e jay na, karon tate password thake.
func postgresURL(s string) error {
	if s == "" {
		return errors.New("nai")
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return errors.New("postgres://user:password@host:port/db akare dao")
	}
	return nil
}

// origin: browser er Origin er moto scheme://host[:port], path chhara. http shudhu local e localhost.
func origin(o, env string) error {
	u, err := url.Parse(o)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("shudhu scheme://host[:port], path chhara")
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && env == "local" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"):
		return nil
	}
	return errors.New("https lagbe (http shudhu APP_ENV=local e localhost)")
}
