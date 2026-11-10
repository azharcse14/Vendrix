package shop

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/db"
)

// Params: `shop-api shop create` er input (3.8, v1.10).
type Params struct {
	Slug, Prefix, Domain  string
	NameBn, NameEn        string
	OwnerEmail, OwnerName string
	Preset                string
}

var (
	slugRE   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	domainRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
)

// Create (3.8): ek transaction e shop (single), primary domain, house vendor, settings (draft = published,
// version 1), order_number counter 0, owner staff (must_change_password). Owner er temporary password
// ferot dey; caller ekbar dekhabe, log e na.
func Create(ctx context.Context, d *sql.DB, p Params) (shopID int64, password string, err error) {
	p.Domain = strings.ToLower(strings.TrimSpace(p.Domain))
	p.NameBn, p.NameEn, p.OwnerName = strings.TrimSpace(p.NameBn), strings.TrimSpace(p.NameEn), strings.TrimSpace(p.OwnerName)
	p.OwnerEmail = strings.ToLower(strings.TrimSpace(p.OwnerEmail))
	user, host, _ := strings.Cut(p.OwnerEmail, "@")
	switch {
	case !slugRE.MatchString(p.Slug) || len(p.Slug) > 80:
		return 0, "", errors.New("--slug: a-z, 0-9 ar - (jemon sobujbazar)")
	case !domainRE.MatchString(p.Domain) || strings.HasPrefix(p.Domain, "www."):
		return 0, "", errors.New("--domain: jemon sobujbazar.com (www chhara, 3.2)")
	case p.NameBn == "":
		return 0, "", errors.New("--name-bn lagbe")
	case p.OwnerName == "":
		return 0, "", errors.New("--owner-name lagbe")
	case user == "" || !strings.Contains(host, "."):
		return 0, "", errors.New("--owner-email bhul")
	}
	name := map[string]string{"bn": p.NameBn}
	if p.NameEn != "" {
		name["en"] = p.NameEn
	}
	settings, err := DefaultSettings(p.Preset, name)
	if err != nil {
		return 0, "", err
	}
	nameJSON, _ := json.Marshal(name)
	password = rand.Text()
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, "", err
	}
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `INSERT INTO shops (slug, name, order_prefix) VALUES ($1, $2, $3) RETURNING id`,
			p.Slug, nameJSON, p.Prefix).Scan(&shopID); err != nil {
			return fmt.Errorf("shop: %w", err)
		}
		for _, q := range []struct {
			what string
			sql  string
			args []any
		}{
			{"domain", `INSERT INTO shop_domains (domain, shop_id, is_primary) VALUES ($1, $2, true)`, []any{p.Domain, shopID}},
			{"house vendor", `INSERT INTO vendors (shop_id, slug, name, is_house) VALUES ($1, $2, $3, true)`, []any{shopID, p.Slug, nameJSON}},
			{"settings", `INSERT INTO shop_settings (shop_id, draft, published) VALUES ($1, $2, $2)`, []any{shopID, settings}},
			{"counter", `INSERT INTO shop_counters (shop_id, name, value) VALUES ($1, 'order_number', 0)`, []any{shopID}},
			{"owner", `INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES ($1, $2, $3, $4, 'owner')`,
				[]any{shopID, p.OwnerEmail, p.OwnerName, hash}},
		} {
			if _, err := tx.ExecContext(ctx, q.sql, q.args...); err != nil {
				return fmt.Errorf("%s: %w", q.what, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, "", err
	}
	return shopID, password, nil
}
