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
	"slices"
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

// Config: M7 porjonto env (13.5). Notun env lagle: ekhane field ar Load e validation, backend/env.example,
// 13.5 er table, config_test.go te test (docs/features/server.md).
type Config struct {
	Env                 string   // APP_ENV: local, staging, production
	Addr                string   // ADDR: shudhu loopback (v1.5); default 127.0.0.1:8080
	DatabaseURL         string   // DATABASE_URL: shop_app role (serve)
	MigrateDatabaseURL  string   // MIGRATE_DATABASE_URL: table er malik (migrate)
	AdminOrigins        []string // ADMIN_ORIGINS: comma diye (CrossOriginProtection, CORS)
	JasprInternalSecret string   // JASPR_INTERNAL_SECRET: Jaspr → API er X-Internal-Auth (3.2)
	CredentialsKey      []byte   // CREDENTIALS_KEY: base64, 32 byte; shop_credentials er AES-GCM (2.2)
	OTPSecret           string   // OTP_SECRET: OTP HMAC, payload AES-GCM er HKDF (8.2)
	TurnstileSecret     string   // TURNSTILE_SECRET: Cloudflare siteverify (12.2)
	TOTPKey             []byte   // TOTP_KEY: base64, 32 byte; staff TOTP secret er AES-GCM (8.3)
	RecoverySecret      string   // RECOVERY_SECRET: recovery code er HMAC (8.3)
	ConfirmSecret       string   // CONFIRM_SECRET: COD confirm token er HMAC (8.10)
	PrintSecret         string   // PRINT_SECRET: print link token er HMAC (11.7 item 8)
	Storage             string   // STORAGE: r2 (default) ba local (shudhu APP_ENV=local; 11.7 item 9)
	UploadDir           string   // UPLOAD_DIR: STORAGE=local er dir; default .data/uploads (process backend/ e chole)
	R2AccountID         string   // R2_ACCOUNT_ID, R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY: STORAGE=r2 e lagbe
	R2Bucket            string
	R2AccessKeyID       string
	R2SecretAccessKey   string
}

// turnstileTestSecrets: Cloudflare er dummy secret (pass, fail, spent); shudhu local e (v1.20).
var turnstileTestSecrets = []string{"1x0000000000000000000000000000000AA", "2x0000000000000000000000000000000AA",
	"3x0000000000000000000000000000000AA"}

// TurnstileTestSecret: Cloudflare er test secret kina; auth e siteverify er hostname check bad dite (10.11 v1.22).
func TurnstileTestSecret(s string) bool { return slices.Contains(turnstileTestSecrets, s) }

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
		c.OTPSecret, c.RecoverySecret, c.ConfirmSecret = get("OTP_SECRET"), get("RECOVERY_SECRET"), get("CONFIRM_SECRET")
		c.PrintSecret = get("PRINT_SECRET")
		for name, v := range map[string]string{"OTP_SECRET": c.OTPSecret, "RECOVERY_SECRET": c.RecoverySecret,
			"CONFIRM_SECRET": c.ConfirmSecret, "PRINT_SECRET": c.PrintSecret} {
			if len(v) < MinSecretLen {
				bad("%s kom-poksha %d okkhor (openssl rand -base64 32)", name, MinSecretLen)
			}
		}
		c.TurnstileSecret = get("TURNSTILE_SECRET")
		if c.TurnstileSecret == "" {
			bad("TURNSTILE_SECRET nai (Cloudflare Turnstile er secret key)")
		}
		if c.Env != "local" && TurnstileTestSecret(c.TurnstileSecret) {
			bad("TURNSTILE_SECRET Cloudflare er test secret; asol secret dao")
		}
		c.TOTPKey = base64Key(get("TOTP_KEY"))
		if c.TOTPKey == nil {
			bad("TOTP_KEY base64 e 32 byte dao (openssl rand -base64 32)")
		}
		c.Storage = get("STORAGE")
		switch c.Storage {
		case "", "r2":
			c.Storage = "r2"
			c.R2AccountID, c.R2Bucket = get("R2_ACCOUNT_ID"), get("R2_BUCKET")
			c.R2AccessKeyID, c.R2SecretAccessKey = get("R2_ACCESS_KEY_ID"), get("R2_SECRET_ACCESS_KEY")
			for _, k := range [][2]string{{"R2_ACCOUNT_ID", c.R2AccountID}, {"R2_BUCKET", c.R2Bucket},
				{"R2_ACCESS_KEY_ID", c.R2AccessKeyID}, {"R2_SECRET_ACCESS_KEY", c.R2SecretAccessKey}} {
				if k[1] == "" {
					bad("%s nai (STORAGE=r2; local e STORAGE=local)", k[0])
				}
			}
		case "local":
			// production e disk e chobi gele backup/CDN er baire thakto
			if c.Env != "local" {
				bad("STORAGE=local shudhu APP_ENV=local e (onno env e r2)")
			}
			c.UploadDir = get("UPLOAD_DIR")
			if c.UploadDir == "" {
				c.UploadDir = ".data/uploads"
			}
		default:
			bad("STORAGE %q ochena (r2 ba local)", c.Storage)
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
			c.CredentialsKey = base64Key(get("CREDENTIALS_KEY"))
			if c.CredentialsKey == nil {
				bad("CREDENTIALS_KEY base64 e 32 byte dao (openssl rand -base64 32)")
			}
		}
	default:
		bad("ochena command %q", cmd)
	}
	// env.example er secret e "local-only" ache; copy kore deploy korle repo-r public secret byabohar hoto
	for name, v := range map[string]string{"JASPR_INTERNAL_SECRET": c.JasprInternalSecret, "CREDENTIALS_KEY": string(c.CredentialsKey),
		"OTP_SECRET": c.OTPSecret, "TOTP_KEY": string(c.TOTPKey), "RECOVERY_SECRET": c.RecoverySecret,
		"CONFIRM_SECRET": c.ConfirmSecret, "PRINT_SECRET": c.PrintSecret} {
		if c.Env != "local" && strings.Contains(v, "local-only") {
			bad("%s env.example er local-only value; notun dao (openssl rand -base64 32)", name)
		}
	}
	return c, errors.Join(errs...)
}

// base64Key: base64 e thik 32 byte (AES-256 key), na hole nil.
func base64Key(s string) []byte {
	k, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(k) != 32 {
		return nil
	}
	return k
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
