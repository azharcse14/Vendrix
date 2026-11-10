package shop

import (
	"context"
	"testing"
	"time"

	"vendrix/backend/internal/dbtest"
)

// TestResolverIntegration (3.2): domain → shop (shops er status, timezone shoho); 60 second cache er
// moddhe ager uttor, por notun; ochena domain cache hoy na (pore jog holei pawa jay).
func TestResolverIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	var id int64
	if err := d.QueryRow(`INSERT INTO shops (slug, name, order_prefix) VALUES ('sobujbazar', '{"bn":"সবুজ"}', 'SB') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO shop_domains (domain, shop_id, is_primary) VALUES ('sobujbazar.com', $1, true)`, id); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r := NewResolver(d)
	r.now = func() time.Time { return now }

	s, ok, err := r.Resolve(ctx, "sobujbazar.com")
	if err != nil || !ok || s.ID != id || s.Slug != "sobujbazar" || s.Status != "active" || s.Mode != "single" || s.Timezone != "Asia/Dhaka" {
		t.Fatalf("%+v %v %v", s, ok, err)
	}
	if _, err := d.Exec(`UPDATE shops SET status = 'suspended' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	now = now.Add(59 * time.Second)
	if s, _, _ := r.Resolve(ctx, "sobujbazar.com"); s.Status != "active" {
		t.Errorf("cache er moddhe: %q", s.Status)
	}
	now = now.Add(2 * time.Second)
	if s, _, _ := r.Resolve(ctx, "sobujbazar.com"); s.Status != "suspended" {
		t.Errorf("60 second por: %q", s.Status)
	}

	if _, ok, err := r.Resolve(ctx, "notun.com"); ok || err != nil {
		t.Fatalf("ochena domain: %v %v", ok, err)
	}
	if _, err := d.Exec(`INSERT INTO shop_domains (domain, shop_id) VALUES ('notun.com', $1)`, id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.Resolve(ctx, "notun.com"); !ok {
		t.Error("ochena domain cache hoyeche: jog korar por-o pawa jay na")
	}
}
