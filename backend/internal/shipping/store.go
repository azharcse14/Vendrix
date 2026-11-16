package shipping

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ErrAreaNotServed: district/area e active zone nai (409 area_not_served).
var ErrAreaNotServed = errors.New("shipping: area e delivery nai")

// Zone: checkout er dorkari field; FreeDeliveryOver 0 = nai (pricing.Zone er mane).
type Zone struct {
	ID               int64
	Charge           int64
	MinOrder         int64
	FreeDeliveryOver int64
	SameCity         bool
}

// DeadlineDays (8.15): delivery_deadline er din; same city 5, na hole 10 (12.2 Digital Commerce Guideline).
// Order ar zone list duijon-i eta dake, client niyom jane na.
func DeadlineDays(sameCity bool) int {
	if sameCity {
		return 5
	}
	return 10
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Lookup (8.19 item 2): area er zone age, na hole district-wide; string hubohu (form e zone er-i string).
// q = *sql.DB ba checkout er *sql.Tx.
func Lookup(ctx context.Context, q querier, shopID int64, district, area string) (Zone, error) {
	var z Zone
	err := q.QueryRowContext(ctx, `SELECT id, charge, min_order, COALESCE(free_delivery_over, 0), same_city
		FROM delivery_zones WHERE shop_id = $1 AND district = $2 AND is_active AND (area = $3 OR area IS NULL)
		ORDER BY area NULLS LAST LIMIT 1`, shopID, district, area).
		Scan(&z.ID, &z.Charge, &z.MinOrder, &z.FreeDeliveryOver, &z.SameCity)
	if errors.Is(err, sql.ErrNoRows) {
		return Zone{}, ErrAreaNotServed
	}
	return z, err
}

// District: GET /api/delivery-zones er ek row (7.2); DistrictWideDays district-wide na hole nil.
type District struct {
	District         string `json:"district"`
	Areas            []Area `json:"areas"`
	DistrictWide     bool   `json:"district_wide"`
	DistrictWideDays *int   `json:"district_wide_days"`
}

type Area struct {
	Area string `json:"area"`
	Name string `json:"name"`
	Days int    `json:"days"`
}

// List: shop er active zone, district ar area er kram e; naam locale e, na thakle bn.
func List(ctx context.Context, d *sql.DB, shopID int64, locale string) ([]District, error) {
	rows, err := d.QueryContext(ctx, `SELECT district, area, name, same_city FROM delivery_zones
		WHERE shop_id = $1 AND is_active ORDER BY district, area NULLS FIRST`, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []District{}
	for rows.Next() {
		var district string
		var area sql.NullString
		var raw []byte
		var sameCity bool
		if err := rows.Scan(&district, &area, &raw, &sameCity); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].District != district {
			out = append(out, District{District: district, Areas: []Area{}})
		}
		cur := &out[len(out)-1]
		days := DeadlineDays(sameCity)
		if !area.Valid {
			cur.DistrictWide, cur.DistrictWideDays = true, &days
			continue
		}
		var name map[string]string
		if err := json.Unmarshal(raw, &name); err != nil {
			return nil, err
		}
		n := name[locale]
		if n == "" {
			n = name["bn"]
		}
		cur.Areas = append(cur.Areas, Area{Area: area.String, Name: n, Days: days})
	}
	return out, rows.Err()
}
