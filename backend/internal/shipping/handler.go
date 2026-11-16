package shipping

import (
	"database/sql"
	"log/slog"
	"net/http"

	"vendrix/backend/internal/httpx"
)

// Handlers: public delivery zone route (7.2).
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// Routes: wrap = public (ShopFromHost → RateLimit).
func (h *Handlers) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/delivery-zones", wrap(http.HandlerFunc(h.Zones)))
}

func (h *Handlers) Zones(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	list, err := List(r.Context(), h.db, s.ID, httpx.Locale(r))
	if err != nil {
		h.log.ErrorContext(r.Context(), "delivery zones", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}
