package catalog

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"unicode/utf8"

	"vendrix/backend/internal/httpx"
)

// Handlers: catalog er public (7.2) ar admin (7.7) route. Public route ShopFromHost er pore boshe.
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "catalog handler", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: fields})
}

type category struct {
	ID       int64      `json:"id"`
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	ImageKey *string    `json:"image_key"`
	Children []category `json:"children,omitempty"`
}

// Categories: GET /api/categories, active category er tree (inactive parent er child o lukano).
func (h *Handlers) Categories(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	rows, err := categories(r.Context(), h.db, s.ID, true)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tree(rows, nil, httpx.Locale(r)))
}

func tree(rows []categoryRow, parent *int64, locale string) []category {
	out := []category{}
	for _, c := range rows {
		if (c.ParentID == nil) != (parent == nil) || (parent != nil && *c.ParentID != *parent) {
			continue
		}
		out = append(out, category{ID: c.ID, Slug: c.Slug, Name: c.Name.In(locale), ImageKey: c.ImageKey,
			Children: tree(rows, &c.ID, locale)})
	}
	return out
}

var sorts = map[string]bool{"newest": true, "price_asc": true, "price_desc": true, "popular": true}

type productCardJSON struct {
	ID             int64   `json:"id"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	Unit           string  `json:"unit"`
	VariableWeight bool    `json:"variable_weight"`
	Price          int64   `json:"price"`
	CompareAtPrice *int64  `json:"compare_at_price"`
	InStock        bool    `json:"in_stock"`
	Rating         float64 `json:"rating"`
	ReviewCount    int64   `json:"review_count"`
	Image          *string `json:"image"`
}

// rating: rating_sum/review_count, ek dosomik (review nai = 0).
func rating(sum, count int64) float64 {
	if count == 0 {
		return 0
	}
	return math.Round(float64(sum)/float64(count)*10) / 10
}

// Products: GET /api/products (7.2): filter, sort, search (6.4), paging (7.1).
func (h *Handlers) Products(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	qs := r.URL.Query()
	f := listFilter{Category: qs.Get("category"), Vendor: qs.Get("vendor"), Sort: qs.Get("sort"),
		Limit: limit, Offset: (page - 1) * limit}
	fields := map[string]string{}
	price := func(name string) *int64 {
		if qs.Get(name) == "" {
			return nil
		}
		n, err := strconv.ParseInt(qs.Get(name), 10, 64)
		if err != nil || n < 0 {
			fields[name] = "invalid"
		}
		return &n
	}
	f.MinPrice, f.MaxPrice = price("min_price"), price("max_price")
	if v := qs.Get("in_stock"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			fields["in_stock"] = "invalid"
		}
		f.InStock = b
	}
	if utf8.RuneCountInString(qs.Get("q")) > 100 {
		fields["q"] = "too_long"
	}
	f.Like, f.Raw, _ = searchTerm(qs.Get("q"))
	switch {
	case f.Sort == "" && f.Raw != "":
		f.Sort = "relevance"
	case f.Sort == "":
		f.Sort = "newest"
	case !sorts[f.Sort]:
		fields["sort"] = "invalid"
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	cards, total, err := listProducts(r.Context(), h.db, s.ID, f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	locale := httpx.Locale(r)
	items := make([]productCardJSON, 0, len(cards))
	for _, c := range cards {
		items = append(items, productCardJSON{ID: c.ID, Slug: c.Slug, Name: c.Name.In(locale), Unit: c.Unit,
			VariableWeight: c.VariableWeight, Price: c.Price, CompareAtPrice: c.CompareAt, InStock: c.InStock,
			Rating: rating(c.RatingSum, c.ReviewCount), ReviewCount: c.ReviewCount, Image: c.Image})
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

type variantJSON struct {
	ID             int64             `json:"id"`
	SKU            string            `json:"sku"`
	Options        map[string]string `json:"options"`
	Price          int64             `json:"price"`
	CompareAtPrice *int64            `json:"compare_at_price"`
	NominalWeightG *int64            `json:"nominal_weight_g"`
	InStock        bool              `json:"in_stock"`
}

type imageJSON struct {
	StorageKey string `json:"storage_key"`
	Alt        string `json:"alt"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type productJSON struct {
	ID             int64           `json:"id"`
	Slug           string          `json:"slug"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Unit           string          `json:"unit"`
	VariableWeight bool            `json:"variable_weight"`
	Category       *string         `json:"category"`
	Attributes     json.RawMessage `json:"attributes"`
	SEO            json.RawMessage `json:"seo"`
	Rating         float64         `json:"rating"`
	ReviewCount    int64           `json:"review_count"`
	Variants       []variantJSON   `json:"variants"`
	Images         []imageJSON     `json:"images"`
}

// Product: GET /api/products/{slug} (7.2). Inactive, active variant chhara ba onno shop er → 404.
func (h *Handlers) Product(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	p, vs, is, err := productBySlug(r.Context(), h.db, s.ID, r.PathValue("slug"))
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	locale := httpx.Locale(r)
	out := productJSON{ID: p.ID, Slug: p.Slug, Name: p.Name.In(locale), Description: p.Description.In(locale),
		Unit: p.Unit, VariableWeight: p.VariableWeight, Category: p.Category, Attributes: p.Attributes,
		SEO: seoIn(p.SEO, locale), Rating: rating(p.RatingSum, p.ReviewCount), ReviewCount: p.ReviewCount,
		Variants: []variantJSON{}, Images: []imageJSON{}}
	for _, v := range vs {
		opts := map[string]string{}
		for k, t := range v.Options {
			opts[k] = t.In(locale)
		}
		out.Variants = append(out.Variants, variantJSON{ID: v.ID, SKU: v.SKU, Options: opts, Price: v.Price,
			CompareAtPrice: v.CompareAt, NominalWeightG: v.NominalWeightG, InStock: v.InStock})
	}
	for _, i := range is {
		out.Images = append(out.Images, imageJSON{StorageKey: i.StorageKey, Alt: i.Alt.In(locale), Width: i.Width, Height: i.Height})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// seoIn: seo {"bn": {...}, "en": {...}} theke locale er ongsho (na thakle bn, tao na thakle {}).
func seoIn(raw json.RawMessage, locale string) json.RawMessage {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	if v, ok := m[locale]; ok {
		return v
	}
	if v, ok := m["bn"]; ok {
		return v
	}
	return json.RawMessage(`{}`)
}
