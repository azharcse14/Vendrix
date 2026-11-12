package shop

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"vendrix/backend/internal/httpx"
)

// Handlers: shop er public route (7.2); ShopFromHost er pore boshe.
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// Settings: GET /api/settings, published er public ongsho (v1.10 whitelist).
func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	published, _, ok := h.settings(w, r)
	if !ok {
		return
	}
	pub, err := PublicSettings(published)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pub)
}

// SettingsVersion: GET /api/settings/version (Jaspr er cache check).
func (h *Handlers) SettingsVersion(w http.ResponseWriter, r *http.Request) {
	if _, version, ok := h.settings(w, r); ok {
		httpx.WriteJSON(w, http.StatusOK, map[string]int{"version": version})
	}
}

func (h *Handlers) settings(w http.ResponseWriter, r *http.Request) ([]byte, int, bool) {
	s, _ := httpx.ShopFrom(r.Context())
	published, version, err := publishedSettings(r.Context(), h.db, s.ID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "shop_not_found"})
		return nil, 0, false
	}
	if err != nil {
		h.fail(w, r, err)
		return nil, 0, false
	}
	return published, version, true
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "shop handler", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}

// Policy: GET /api/policies/{kind} (7.2, v1.13): `{"kind", "body"}`, body plain text (10.10); ochena kind 404.
func (h *Handlers) Policy(w http.ResponseWriter, r *http.Request) {
	published, _, ok := h.settings(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	body, found, err := PolicyText(published, kind, httpx.Locale(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if !found {
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"kind": kind, "body": body})
}

// Translations: GET /api/translations?locale=bn|en, default file er upore shop er override (7.2, 9.8).
func (h *Handlers) Translations(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	if locale != "bn" && locale != "en" {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{"locale": "invalid"}})
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	t, err := translations(r.Context(), h.db, s.ID, locale)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}
