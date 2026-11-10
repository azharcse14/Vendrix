package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadMigrations(t *testing.T) {
	ok := fstest.MapFS{
		"002_more.sql":            {Data: []byte("SELECT 2;")},
		"001_init.sql":            {Data: []byte("SELECT 1;")},
		"tests/001_init_test.sql": {Data: []byte("x")},
		"DEPLOYED":                {Data: []byte("# x")},
	}
	ms, err := LoadMigrations(ok)
	if err != nil || len(ms) != 2 || ms[0].Version != "001_init" || ms[1].Version != "002_more" {
		t.Fatalf("%+v %v", ms, err)
	}
	if want := fmt.Sprintf("%x", sha256.Sum256([]byte("SELECT 1;"))); ms[0].SHA256 != want || ms[0].SQL != "SELECT 1;" {
		t.Errorf("sha256 %s, want %s", ms[0].SHA256, want)
	}
	bad := map[string]fstest.MapFS{
		"test file top-level e": {"001_init.sql": {Data: []byte("x")}, "001_init_test.sql": {Data: []byte("x")}},
		"boro hater naam":       {"001_Init.sql": {Data: []byte("x")}},
		"number chhara":         {"init.sql": {Data: []byte("x")}},
		"ekoi number duibar":    {"001_a.sql": {Data: []byte("x")}, "001_b.sql": {Data: []byte("y")}},
		"khali":                 {},
	}
	for name, fsys := range bad {
		if _, err := LoadMigrations(fsys); err == nil {
			t.Errorf("%s: error ashe ni", name)
		}
	}
}

// TestPending (6.1 v1.7, 14.2 test 42 er unit ongsho).
func TestPending(t *testing.T) {
	files := []Migration{{Version: "001_init", SHA256: "a"}, {Version: "002_x", SHA256: "b"}, {Version: "003_y", SHA256: "c"}}
	for _, c := range []struct {
		applied map[string]string
		want    []string
	}{
		{nil, []string{"001_init", "002_x", "003_y"}},
		{map[string]string{"001_init": "a"}, []string{"002_x", "003_y"}},
		{map[string]string{"001_init": "a", "002_x": "b", "003_y": "c"}, nil},
	} {
		p, err := Pending(files, c.applied)
		var got []string
		for _, m := range p {
			got = append(got, m.Version)
		}
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("applied %v: %v %v, want %v", c.applied, got, err, c.want)
		}
	}
	for _, c := range []struct {
		name    string
		applied map[string]string
		want    string
	}{
		{"applied file bodleche", map[string]string{"001_init": "z"}, "deploy er por bodleche"},
		{"applied file nai", map[string]string{"001_init": "a", "000_old": "q"}, "file nai"},
		{"out-of-order notun file", map[string]string{"001_init": "a", "003_y": "c"}, "choto number"},
	} {
		if _, err := Pending(files, c.applied); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

type sqlStateErr string

func (e sqlStateErr) Error() string    { return "pg: " + string(e) }
func (e sqlStateErr) SQLState() string { return string(e) }

func TestRetryable(t *testing.T) {
	for code, want := range map[string]bool{"40P01": true, "40001": true, "23505": false} {
		if got := retryable(fmt.Errorf("wrap: %w", sqlStateErr(code))); got != want {
			t.Errorf("%s: %v", code, got)
		}
	}
	if retryable(errors.New("x")) || retryable(nil) {
		t.Error("SQLState chhara error e retry na")
	}
}

func TestOpenWithoutDriver(t *testing.T) {
	if slices.Contains(sql.Drivers(), DriverName) {
		t.Skip("pgx driver ache (M0 er driver step shesh): Open er asol test TestMigrateIntegration e")
	}
	if _, err := Open(context.Background(), "postgres://x@127.0.0.1:1/db"); !errors.Is(err, ErrNoDriver) {
		t.Errorf("err = %v, want ErrNoDriver", err)
	}
}
