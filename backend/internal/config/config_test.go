package config

import (
	"os"
	"strings"
	"testing"
)

func env(kv ...string) func(string) (string, bool) {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

const secret = "0123456789abcdef0123456789abcdef"

// totpKey: base64 e 32 byte.
const totpKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

// authEnv: serve er M4 secret (13.5).
var authEnv = []string{"OTP_SECRET", secret, "TURNSTILE_SECRET", "0x4AAAAAAAreal-turnstile-secret", "TOTP_KEY", totpKey,
	"RECOVERY_SECRET", secret}

func TestLoadServe(t *testing.T) {
	c, err := Load(env(append([]string{"APP_ENV", "production", "DATABASE_URL", "postgres://shop_app:x@127.0.0.1:5432/shop",
		"JASPR_INTERNAL_SECRET", secret, "ADMIN_ORIGINS", "https://admin.tomarshop.com, https://admin2.tomarshop.com"}, authEnv...)...), Serve)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != "127.0.0.1:8080" || len(c.AdminOrigins) != 2 || c.AdminOrigins[1] != "https://admin2.tomarshop.com" {
		t.Errorf("config: %+v", c)
	}
	if c.OTPSecret != secret || c.RecoverySecret != secret || len(c.TOTPKey) != 32 || c.TurnstileSecret == "" {
		t.Errorf("auth secret: %+v", c)
	}
	// local e http://localhost er admin cholbe, ADMIN_ORIGINS chhara-o
	for _, origins := range []string{"http://localhost:3001", ""} {
		if _, err := Load(env(append([]string{"APP_ENV", "local", "DATABASE_URL", "postgres://a@localhost/db", "JASPR_INTERNAL_SECRET", secret,
			"ADMIN_ORIGINS", origins, "ADDR", "localhost:9090"}, authEnv...)...), Serve); err != nil {
			t.Errorf("local %q: %v", origins, err)
		}
	}
}

// TestLoadRejects: protiti bhul alada dhora pore, ar error e kokhono password na.
func TestLoadRejects(t *testing.T) {
	base := append([]string{"APP_ENV", "production", "DATABASE_URL", "postgres://a:pw-secret@h/db", "JASPR_INTERNAL_SECRET", secret,
		"ADMIN_ORIGINS", "https://admin.x.com"}, authEnv...)
	with := func(k, v string) func(string) (string, bool) {
		kv := append([]string{}, base...)
		for i := 0; i < len(kv); i += 2 {
			if kv[i] == k {
				kv[i+1] = v
				return env(kv...)
			}
		}
		return env(append(kv, k, v)...)
	}
	cases := []struct {
		name   string
		lookup func(string) (string, bool)
		want   string
	}{
		{"APP_ENV nai", with("APP_ENV", ""), "APP_ENV nai"},
		{"APP_ENV ochena", with("APP_ENV", "prod"), "ochena"},
		{"ADDR shob interface e", with("ADDR", ":8080"), "loopback na"},
		{"ADDR public IP", with("ADDR", "10.0.0.5:8080"), "loopback na"},
		{"ADDR port chhara", with("ADDR", "127.0.0.1"), "host:port"},
		{"DATABASE_URL nai", with("DATABASE_URL", ""), "DATABASE_URL: nai"},
		{"DATABASE_URL mysql", with("DATABASE_URL", "mysql://a:pw-secret@h/db"), "DATABASE_URL"},
		{"secret choto", with("JASPR_INTERNAL_SECRET", "short"), "JASPR_INTERNAL_SECRET"},
		{"production e ADMIN_ORIGINS nai", with("ADMIN_ORIGINS", ""), "ADMIN_ORIGINS nai"},
		{"production e http origin", with("ADMIN_ORIGINS", "http://localhost:3001"), "https lagbe"},
		{"origin e path", with("ADMIN_ORIGINS", "https://admin.x.com/app"), "path chhara"},
		{"OTP_SECRET nai", with("OTP_SECRET", ""), "OTP_SECRET"},
		{"OTP_SECRET choto", with("OTP_SECRET", "short"), "OTP_SECRET"},
		{"RECOVERY_SECRET nai", with("RECOVERY_SECRET", ""), "RECOVERY_SECRET"},
		{"TURNSTILE_SECRET nai", with("TURNSTILE_SECRET", ""), "TURNSTILE_SECRET"},
		{"TOTP_KEY nai", with("TOTP_KEY", ""), "TOTP_KEY"},
		{"TOTP_KEY base64 na", with("TOTP_KEY", "pw-secret not base64!"), "TOTP_KEY"},
		{"TOTP_KEY 16 byte", with("TOTP_KEY", "MDEyMzQ1Njc4OWFiY2RlZg=="), "TOTP_KEY"},
		// v1.20: Cloudflare er test secret production e Turnstile bondho kore dito
		{"Turnstile pass test secret", with("TURNSTILE_SECRET", "1x0000000000000000000000000000000AA"), "TURNSTILE_SECRET"},
		{"Turnstile fail test secret", with("TURNSTILE_SECRET", "2x0000000000000000000000000000000AA"), "TURNSTILE_SECRET"},
		{"Turnstile spent test secret", with("TURNSTILE_SECRET", "3x0000000000000000000000000000000AA"), "TURNSTILE_SECRET"},
	}
	for _, c := range cases {
		_, err := Load(c.lookup, Serve)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
		if err != nil && strings.Contains(err.Error(), "pw-secret") {
			t.Errorf("%s: error e password: %v", c.name, err)
		}
	}
}

func TestLoadMigrate(t *testing.T) {
	c, err := Load(env("APP_ENV", "staging", "MIGRATE_DATABASE_URL", "postgresql://owner:x@db:5432/shop"), Migrate)
	if err != nil || c.MigrateDatabaseURL == "" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := Load(env("APP_ENV", "staging"), Migrate); err == nil || !strings.Contains(err.Error(), "MIGRATE_DATABASE_URL") {
		t.Errorf("MIGRATE_DATABASE_URL chhara: %v", err)
	}
	if _, err := Load(env("APP_ENV", "local"), Command("seed")); err == nil {
		t.Error("ochena command e error ashe ni")
	}
}

// TestLoadCredentials (12.2, 13.5): CREDENTIALS_KEY base64 e thik 32 byte; error e key thake na.
func TestLoadCredentials(t *testing.T) {
	key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // 32 byte
	c, err := Load(env("APP_ENV", "local", "DATABASE_URL", "postgres://shop_app:x@127.0.0.1:5433/shop",
		"CREDENTIALS_KEY", key), Credentials)
	if err != nil || len(c.CredentialsKey) != 32 || c.DatabaseURL == "" {
		t.Fatalf("%v %v", len(c.CredentialsKey), err)
	}
	for _, bad := range []string{"", "not base64!", "MDEyMzQ1Njc4OWFiY2RlZg=="} { // shesh ta 16 byte
		_, err := Load(env("APP_ENV", "local", "DATABASE_URL", "postgres://shop_app:x@127.0.0.1:5433/shop",
			"CREDENTIALS_KEY", bad), Credentials)
		if err == nil || !strings.Contains(err.Error(), "CREDENTIALS_KEY") || (bad != "" && strings.Contains(err.Error(), bad)) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, err := Load(env("APP_ENV", "local", "CREDENTIALS_KEY", key), Credentials); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("DATABASE_URL chhara: %v", err)
	}
}

// TestRejectExampleSecrets (13.5): env.example er "local-only" secret staging/production e cholbe na.
func TestRejectExampleSecrets(t *testing.T) {
	exampleKey := "bG9jYWwtb25seS1rZXktbm90LWZvci1wcm9kLTAwMDE=" // "local-only-key-not-for-prod-0001"
	base := []string{"DATABASE_URL", "postgres://shop_app:x@127.0.0.1:5433/shop"}
	if _, err := Load(env(append(base, "APP_ENV", "local", "CREDENTIALS_KEY", exampleKey)...), Credentials); err != nil {
		t.Errorf("local e example key: %v", err)
	}
	if _, err := Load(env(append(base, "APP_ENV", "production", "CREDENTIALS_KEY", exampleKey)...), Credentials); err == nil ||
		!strings.Contains(err.Error(), "CREDENTIALS_KEY") {
		t.Errorf("production e example key: %v", err)
	}
	// backend/env.example nijei: local e chole, staging e protiti local-only secret ar Turnstile test secret atke
	example := envFile(t, "../../env.example")
	if _, err := Load(example, Serve); err != nil {
		t.Errorf("local e env.example: %v", err)
	}
	staging := func(k string) (string, bool) {
		switch k {
		case "APP_ENV":
			return "staging", true
		case "ADMIN_ORIGINS":
			return "https://admin.tomarshop.com", true
		}
		return example(k)
	}
	_, err := Load(staging, Serve)
	for _, k := range []string{"JASPR_INTERNAL_SECRET", "OTP_SECRET", "TOTP_KEY", "RECOVERY_SECRET", "TURNSTILE_SECRET"} {
		if err == nil || !strings.Contains(err.Error(), k) {
			t.Errorf("staging e env.example er %s: %v", k, err)
		}
	}
}

// envFile: KEY=VALUE file (# comment bad) theke lookup.
func envFile(t *testing.T, path string) func(string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var kv []string
	for _, l := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(l, "="); ok && !strings.HasPrefix(strings.TrimSpace(l), "#") {
			kv = append(kv, strings.TrimSpace(k), v)
		}
	}
	return env(kv...)
}
