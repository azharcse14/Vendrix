package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// StaffAdmin: staff management (7.7, 11.7 item 11, v1.46), shudhu owner.
type StaffAdmin struct {
	db  *sql.DB
	log *slog.Logger
}

func NewStaffAdmin(d *sql.DB, log *slog.Logger) *StaffAdmin { return &StaffAdmin{db: d, log: log} }

func (h *StaffAdmin) AdminRoutes(mux *http.ServeMux, staff httpx.Middleware) {
	owner := func(f http.HandlerFunc) http.Handler { return staff(httpx.RequireRole("owner")(f)) }
	mux.Handle("GET /api/admin/staff", owner(h.List))
	mux.Handle("POST /api/admin/staff", owner(h.Create))
	mux.Handle("PATCH /api/admin/staff/{id}", owner(h.Update))
	mux.Handle("POST /api/admin/staff/{id}/reset", owner(h.Reset))
}

// shopRoles: vendor role MV porjonto na (staff_vendor_role_check e vendor_id o lage).
var shopRoles = []string{"owner", "admin", "staff"}

var errStaffForbidden = errors.New("staff: nijeke ba owner na")

type fieldErr struct{ field string }

func (e fieldErr) Error() string { return "staff: bhul " + e.field }

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// staffRows: list ba ek staff (id > 0) er row; hash, secret, failed_* na; locked_until ager hole null. Purono age.
func staffRows(ctx context.Context, q querier, shopID, id int64, limit, offset int) ([]json.RawMessage, int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT jsonb_build_object('id', id, 'email', email, 'name', name, 'role', role,
		'vendor_id', vendor_id, 'is_active', is_active, 'must_change_password', must_change_password,
		'totp_enabled', totp_secret_enc IS NOT NULL, 'locked_until', CASE WHEN locked_until > now() THEN locked_until END,
		'last_login_at', last_login_at, 'created_at', created_at), count(*) OVER ()
		FROM staff WHERE shop_id = $1 AND ($2::bigint = 0 OR id = $2) ORDER BY created_at, id LIMIT $3 OFFSET $4`,
		shopID, id, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []json.RawMessage
	var total int64
	for rows.Next() {
		var v json.RawMessage
		if err := rows.Scan(&v, &total); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}

// staffOne: ek staff er row (na thakle sql.ErrNoRows).
func staffOne(ctx context.Context, q querier, shopID, id int64) (json.RawMessage, error) {
	items, _, err := staffRows(ctx, q, shopID, id, 1, 0)
	if err == nil && len(items) == 0 {
		err = sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return items[0], nil
}

func (h *StaffAdmin) fail(w http.ResponseWriter, r *http.Request, err error) {
	var fe fieldErr
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &fe):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{fe.field: "invalid"}})
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
	case errors.Is(err, errStaffForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "forbidden"})
	case errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "staff_email_uniq":
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "staff_exists"})
	default:
		// err e kokhono temp password thake na (hash/SQL error)
		h.log.ErrorContext(r.Context(), "staff admin", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

// List: GET /api/admin/staff?page=&limit=, purono age.
func (h *StaffAdmin) List(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	items, total, err := staffRows(r.Context(), h.db, st.ShopID, 0, limit, (page-1)*limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

func checkName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 1 || n > 100 {
		return "", fieldErr{"name"}
	}
	return s, nil
}

func checkRole(s string) error {
	if !slices.Contains(shopRoles, s) {
		return fieldErr{"role"}
	}
	return nil
}

// tempPassword: 26 base32 (128 bit, 12–128 er moddhe); hash tx er age, PBKDF2 semaphore e (owner row lock dhore na).
func tempPassword(ctx context.Context) (string, string, error) {
	pw := rand.Text()
	hash, err := hashPasswordSem(ctx, pw)
	return pw, hash, err
}

// withPassword: row e temp_password (response e ekbar; audit/log e na).
func withPassword(row json.RawMessage, pw string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(row, &m); err != nil {
		return nil, err
	}
	m["temp_password"], _ = json.Marshal(pw)
	return json.Marshal(m)
}

func writeSecret(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// logStaff: audit (entity staff, target id); insertAudit, audit package na (shop → auth import cycle).
func logStaff(r *http.Request, tx *sql.Tx, st httpx.Staff, action string, id int64, before, after map[string]any) error {
	return insertAudit(r.Context(), tx, audit{shopID: st.ShopID, staffID: st.ID, entity: id, action: action,
		ip: httpx.IP(r.Context()), before: before, after: after})
}

// Create: POST /api/admin/staff {email, name, role} → 201 row + temp_password; must_change_password (8.3 step 4).
func (h *StaffAdmin) Create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Name, Role string }
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		h.fail(w, r, fieldErr{"body"})
		return
	}
	email := strings.TrimSpace(in.Email)
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email || len(email) > 254 {
		h.fail(w, r, fieldErr{"email"})
		return
	}
	name, err := checkName(in.Name)
	if err == nil {
		err = checkRole(in.Role)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ctx := r.Context()
	pw, hash, err := tempPassword(ctx)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	st, _ := httpx.StaffFrom(ctx)
	var row json.RawMessage
	err = db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		var id int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO staff (shop_id, email, name, password_hash, role, must_change_password)
			VALUES ($1, $2, $3, $4, $5, true) RETURNING id`, st.ShopID, email, name, hash, in.Role).Scan(&id); err != nil {
			return err
		}
		var err error
		if row, err = staffOne(ctx, tx, st.ShopID, id); err != nil {
			return err
		}
		return logStaff(r, tx, st, "staff.create", id, nil, map[string]any{"email": email, "name": name, "role": in.Role})
	})
	var b []byte
	if err == nil {
		b, err = withPassword(row, pw)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeSecret(w, http.StatusCreated, b)
}

// lockOwners: shop er active owner (id kram e) FOR UPDATE; caller oi set e na thakle errStaffForbidden. Duijon
// owner ek shathe ekjon arek jon ke namale porer jon lock er por nijeke set e pay na (v1.46).
func lockOwners(ctx context.Context, tx *sql.Tx, st httpx.Staff) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner' AND is_active ORDER BY id FOR UPDATE`, st.ShopID)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		found = found || id == st.ID
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return errStaffForbidden
	}
	return nil
}

func staffID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, sql.ErrNoRows
	}
	return id, nil
}

// Update: PATCH /api/admin/staff/{id} {name?, role?, is_active?} → 200 row. Nijer role/is_active bodol 403;
// role ba is_active asolei bodlale oi staff er shob session revoke (12.2 item 5).
func (h *StaffAdmin) Update(w http.ResponseWriter, r *http.Request) {
	id, err := staffID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var in struct {
		Name     *string `json:"name"`
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		h.fail(w, r, fieldErr{"body"})
		return
	}
	if in.Name != nil {
		n, err := checkName(*in.Name)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		in.Name = &n
	}
	if in.Role != nil {
		if err := checkRole(*in.Role); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	ctx := r.Context()
	st, _ := httpx.StaffFrom(ctx)
	var row json.RawMessage
	err = db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		if err := lockOwners(ctx, tx, st); err != nil {
			return err
		}
		var name, role string
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT name, role, is_active FROM staff WHERE shop_id = $1 AND id = $2 FOR UPDATE`,
			st.ShopID, id).Scan(&name, &role, &active); err != nil {
			return err
		}
		before, after := map[string]any{}, map[string]any{}
		if in.Name != nil && *in.Name != name {
			before["name"], after["name"], name = name, *in.Name, *in.Name
		}
		if in.Role != nil && *in.Role != role {
			before["role"], after["role"], role = role, *in.Role, *in.Role
		}
		if in.IsActive != nil && *in.IsActive != active {
			before["is_active"], after["is_active"], active = active, *in.IsActive, *in.IsActive
		}
		_, roleChanged := after["role"]
		_, activeChanged := after["is_active"]
		if id == st.ID && (roleChanged || activeChanged) {
			return errStaffForbidden
		}
		if len(after) > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE staff SET name = $3, role = $4, is_active = $5 WHERE shop_id = $1 AND id = $2`,
				st.ShopID, id, name, role, active); err != nil {
				return err
			}
			if roleChanged || activeChanged {
				if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = now()
					WHERE shop_id = $1 AND staff_id = $2 AND revoked_at IS NULL`, st.ShopID, id); err != nil {
					return err
				}
			}
			if err := logStaff(r, tx, st, "staff.update", id, before, after); err != nil {
				return err
			}
		}
		var err error
		row, err = staffOne(ctx, tx, st.ShopID, id)
		return err
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(row)
}

// Reset: POST /api/admin/staff/{id}/reset → 200 {temp_password}: notun password, must_change_password, lock khole,
// TOTP ar recovery code muche, shob session revoke (11.7 item 11). Nijeke 403 (recovery code ba CLI, 8.3).
func (h *StaffAdmin) Reset(w http.ResponseWriter, r *http.Request) {
	id, err := staffID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ctx := r.Context()
	st, _ := httpx.StaffFrom(ctx)
	if id == st.ID {
		h.fail(w, r, errStaffForbidden)
		return
	}
	pw, hash, err := tempPassword(ctx)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	err = db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		if err := lockOwners(ctx, tx, st); err != nil {
			return err
		}
		ok, err := affected(tx.ExecContext(ctx, `UPDATE staff SET password_hash = $3, must_change_password = true,
			failed_logins = 0, locked_until = NULL WHERE shop_id = $1 AND id = $2`, st.ShopID, id, hash))
		if err != nil {
			return err
		}
		if !ok {
			return sql.ErrNoRows
		}
		if err := clearStaff2FA(ctx, tx, st.ShopID, id); err != nil {
			return err
		}
		return logStaff(r, tx, st, "staff.reset", id, nil, map[string]any{"must_change_password": true, "totp_cleared": true})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	b, _ := json.Marshal(map[string]string{"temp_password": pw})
	writeSecret(w, http.StatusOK, b)
}
