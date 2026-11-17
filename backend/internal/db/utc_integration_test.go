package db_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/dbtest"
)

// TestOpenUTCIntegration (7.1): database er timezone Asia/Dhaka holeo db.Open er connection e to_jsonb
// er timestamptz ar Go te scan kora time.Time UTC. Shob caller (admin JSON, order view) ei pool diye.
func TestOpenUTCIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	var name string
	if err := d.QueryRowContext(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	// rule12:ok database er naam dbtest er random
	if _, err := d.ExecContext(ctx, `ALTER DATABASE `+name+` SET timezone = 'Asia/Dhaka'`); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	u.Path = "/" + name
	p, err := db.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	var tz, js string
	var at time.Time
	if err := p.QueryRowContext(ctx, `SELECT current_setting('TimeZone'), to_jsonb(now())::text, now()`).Scan(&tz, &js, &at); err != nil {
		t.Fatal(err)
	}
	if tz != "UTC" {
		t.Errorf("session TimeZone %q, want UTC", tz)
	}
	if !strings.HasSuffix(js, `+00:00"`) {
		t.Errorf("to_jsonb %s, want +00:00", js)
	}
	if at.Location() != time.UTC {
		t.Errorf("scan location %v, want UTC", at.Location())
	}
	if b, _ := json.Marshal(at); !strings.HasSuffix(string(b), `Z"`) {
		t.Errorf("Go JSON %s, want Z", b)
	}
}
