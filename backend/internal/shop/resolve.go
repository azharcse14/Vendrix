package shop

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"vendrix/backend/internal/httpx"
)

// cacheTTL (3.2): domain → shop er memory cache; status bodlale (suspended) shorboccho eto deri.
const cacheTTL = 60 * time.Second

// Resolver: shop_domains theke shop (httpx.ShopFromHost er ResolveShop). Shudhu pawa domain cache hoy,
// tai ochena X-Shop-Host diye memory bharano jay na. Ochena host e protibar DB lookup, rate limit er age
// (key e shop lage); shima 16.1 er proshno.
type Resolver struct {
	db    *sql.DB
	now   func() time.Time
	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	shop    httpx.Shop
	expires time.Time
}

func NewResolver(d *sql.DB) *Resolver {
	return &Resolver{db: d, now: time.Now, cache: map[string]cached{}}
}

// Resolve: domain lowercase, port chhara (httpx.ShopFromHost dey).
func (r *Resolver) Resolve(ctx context.Context, domain string) (httpx.Shop, bool, error) {
	r.mu.Lock()
	c, hit := r.cache[domain]
	r.mu.Unlock()
	if hit && r.now().Before(c.expires) {
		return c.shop, true, nil
	}
	var s httpx.Shop
	err := r.db.QueryRowContext(ctx, `SELECT s.id, s.slug, s.mode, s.status, s.timezone
		FROM shop_domains d JOIN shops s ON s.id = d.shop_id WHERE d.domain = $1`, domain).
		Scan(&s.ID, &s.Slug, &s.Mode, &s.Status, &s.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return httpx.Shop{}, false, nil
	}
	if err != nil {
		return httpx.Shop{}, false, err
	}
	r.mu.Lock()
	r.cache[domain] = cached{shop: s, expires: r.now().Add(cacheTTL)}
	r.mu.Unlock()
	return s, true, nil
}
