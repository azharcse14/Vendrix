package customer

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/phone"
)

type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// AdminRoutes (7.7): customer data PII, tai shudhu owner/admin (11.2, v1.45). Unblock e phone body te
// (URL/access log e na, 11.7 item 12).
func (h *Handlers) AdminRoutes(mux *http.ServeMux, staff httpx.Middleware) {
	admin := func(f http.HandlerFunc) http.Handler { return staff(httpx.RequireRole("owner", "admin")(f)) }
	mux.Handle("GET /api/admin/customers", admin(h.Customers))
	mux.Handle("GET /api/admin/customers/{id}", admin(h.Customer))
	mux.Handle("GET /api/admin/blocked-phones", admin(h.Blocked))
	mux.Handle("POST /api/admin/blocked-phones", admin(h.Block))
	mux.Handle("DELETE /api/admin/blocked-phones", admin(h.Unblock))
}

func (h *Handlers) invalid(w http.ResponseWriter, r *http.Request, field string) {
	httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{field: "invalid"}})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
		return
	}
	h.log.ErrorContext(r.Context(), "customer admin", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}

func (h *Handlers) paged(w http.ResponseWriter, r *http.Request, items []json.RawMessage, page, limit int, total int64, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// Customers: GET /api/admin/customers?q=&page=&limit=; q phone (normalize hole hubohu) ba naam er ongsho, max 64.
func (h *Handlers) Customers(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	var ph, name any
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		if utf8.RuneCountInString(q) > 64 {
			h.invalid(w, r, "q")
			return
		}
		if p, err := phone.Normalize(q); err == nil {
			ph = p
		} else {
			name = q
		}
	}
	st, _ := httpx.StaffFrom(r.Context())
	items, total, err := listCustomers(r.Context(), h.db, st.ShopID, ph, name, limit, (page-1)*limit)
	h.paged(w, r, items, page, limit, total, err)
}

// Customer: GET /api/admin/customers/{id}: profile, blocked, shesh 50 order.
func (h *Handlers) Customer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.fail(w, r, sql.ErrNoRows)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	c, err := customerDetail(r.Context(), h.db, st.ShopID, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(c)
}

// Blocked: GET /api/admin/blocked-phones?page=&limit=, notun age.
func (h *Handlers) Blocked(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	items, total, err := listBlocked(r.Context(), h.db, st.ShopID, limit, (page-1)*limit)
	h.paged(w, r, items, page, limit, total, err)
}

type blockIn struct {
	Phone  string  `json:"phone"`
	Reason *string `json:"reason"`
}

// decodeBlock: phone normalize (Rule 6), reason trim max 200 (khali = NULL).
func (h *Handlers) decodeBlock(w http.ResponseWriter, r *http.Request, withReason bool) (string, any, bool) {
	var in blockIn
	if err := httpx.DecodeJSON(w, r, &in); err != nil || (!withReason && in.Reason != nil) {
		h.invalid(w, r, "body")
		return "", nil, false
	}
	p, err := phone.Normalize(in.Phone)
	if err != nil {
		h.invalid(w, r, "phone")
		return "", nil, false
	}
	var reason any
	if in.Reason != nil {
		s := strings.TrimSpace(*in.Reason)
		if utf8.RuneCountInString(s) > 200 {
			h.invalid(w, r, "reason")
			return "", nil, false
		}
		if s != "" {
			reason = s
		}
	}
	return p, reason, true
}

// Block: POST /api/admin/blocked-phones {phone, reason?} → notun 201, already blocked 200 ager row (write na).
func (h *Handlers) Block(w http.ResponseWriter, r *http.Request) {
	p, reason, ok := h.decodeBlock(w, r, true)
	if !ok {
		return
	}
	ctx := r.Context()
	st, _ := httpx.StaffFrom(ctx)
	var row json.RawMessage
	created := false
	err := db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		var err error
		row, created, err = block(ctx, tx, st.ShopID, st.ID, p, reason)
		if err != nil || !created {
			return err
		}
		return h.audit(r, tx, st, "customer.block", p, nil, auditJSON(reason, p))
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(row)
}

// Unblock: DELETE /api/admin/blocked-phones {phone} → 204 (na thakleo; audit shudhu muchle).
func (h *Handlers) Unblock(w http.ResponseWriter, r *http.Request) {
	p, _, ok := h.decodeBlock(w, r, false)
	if !ok {
		return
	}
	ctx := r.Context()
	st, _ := httpx.StaffFrom(ctx)
	err := db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		reason, found, err := unblock(ctx, tx, st.ShopID, p)
		if err != nil || !found {
			return err
		}
		return h.audit(r, tx, st, "customer.unblock", p, auditJSON(reason, p), nil)
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// auditJSON: reason ar masked phone (12.2); puro phone na, kintu guest phone er unblock o chena jay.
func auditJSON(reason any, p string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"reason": reason, "phone_masked": phone.Mask(p)})
	return b
}

// audit: entity oi phone er user (na thakle 0); before/after e puro phone kokhono na (12.2, v1.45).
func (h *Handlers) audit(r *http.Request, tx *sql.Tx, st httpx.Staff, action, p string, before, after json.RawMessage) error {
	uid, err := userByPhone(r.Context(), tx, st.ShopID, p)
	if err != nil {
		return err
	}
	return audit.Log(r.Context(), tx, audit.Entry{ShopID: st.ShopID, StaffID: st.ID, Action: action, EntityType: "customer",
		EntityID: uid, Before: before, After: after, IP: httpx.IP(r.Context())})
}
