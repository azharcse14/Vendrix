package audit

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"vendrix/backend/internal/httpx"
)

// Handlers: GET /api/admin/audit-logs (7.7, 11.7 item 11), read-only.
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// AdminRoutes: owner/admin (11.2).
func (h *Handlers) AdminRoutes(mux *http.ServeMux, staff httpx.Middleware) {
	mux.Handle("GET /api/admin/audit-logs", staff(httpx.RequireRole("owner", "admin")(http.HandlerFunc(h.List))))
}

// List: ?action= (hubohu, max 64) &staff_id= &from=&to= (YYYY-MM-DD shop timezone e, to shoho) &page=&limit=;
// notun age. staff_name LEFT JOIN: CLI row e staff nai.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	action, from, to := q.Get("action"), q.Get("from"), q.Get("to")
	var staffID int64
	field := ""
	if len(action) > 64 {
		field = "action"
	}
	if s := q.Get("staff_id"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			field = "staff_id"
		}
		staffID = n
	}
	for name, v := range map[string]string{"from": from, "to": to} {
		if v != "" && !httpx.ValidDate(v) {
			field = name
		}
	}
	if field != "" {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{field: "invalid"}})
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	rows, err := h.db.QueryContext(r.Context(), `SELECT jsonb_build_object('id', a.id, 'staff_id', a.staff_id, 'staff_name', s.name,
			'action', a.action, 'entity_type', a.entity_type, 'entity_id', a.entity_id, 'before', a.before, 'after', a.after,
			'ip', a.ip, 'created_at', a.created_at), count(*) OVER ()
		FROM audit_logs a LEFT JOIN staff s ON s.shop_id = a.shop_id AND s.id = a.staff_id
		WHERE a.shop_id = $1 AND ($2 = '' OR a.action = $2) AND ($3::bigint = 0 OR a.staff_id = $3)
		AND ($4 = '' OR a.created_at >= NULLIF($4, '')::date::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1))
		AND ($5 = '' OR a.created_at < (NULLIF($5, '')::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1))
		ORDER BY a.created_at DESC, a.id DESC LIMIT $6 OFFSET $7`, st.ShopID, action, staffID, from, to, limit, (page-1)*limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	defer rows.Close()
	var items []json.RawMessage
	var total int64
	for rows.Next() {
		var v json.RawMessage
		if err := rows.Scan(&v, &total); err != nil {
			h.fail(w, r, err)
			return
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "audit list", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}
