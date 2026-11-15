package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/jobs"
)

type loginRow struct {
	shopID, id      int64 // id 0 = ei shop e email nai
	hash            string
	active, hasTOTP bool
	locked          time.Duration // locked_until baki; 0 = lock nai
}

// staffForLogin (8.3 step 1): slug er shop (status jai hok, Q7) ar tar staff (email boro/choto haat mane na,
// staff_email_uniq). Shop na thakle sql.ErrNoRows.
func staffForLogin(ctx context.Context, d *sql.DB, slug, email string) (loginRow, error) {
	var r loginRow
	var secs float64
	err := d.QueryRowContext(ctx, `SELECT sh.id, coalesce(st.id, 0), coalesce(st.password_hash, ''),
		  coalesce(st.is_active, false), st.totp_secret_enc IS NOT NULL,
		  coalesce(greatest(extract(epoch FROM st.locked_until - now()), 0), 0)
		FROM shops sh LEFT JOIN staff st ON st.shop_id = sh.id AND lower(st.email) = lower($2)
		WHERE sh.slug = $1`, slug, email).Scan(&r.shopID, &r.id, &r.hash, &r.active, &r.hasTOTP, &secs)
	r.locked = time.Duration(secs * float64(time.Second))
	return r, err
}

// bumpFailed: failed_logins (mfa false) ba failed_mfa + 1; notun gona after er shoman ba beshi hole locked_until
// = now + lock. Notun gona ferot.
func bumpFailed(ctx context.Context, tx *sql.Tx, shopID, staffID int64, mfa bool, after int, lock time.Duration) (int, error) {
	q := `UPDATE staff SET failed_logins = failed_logins + 1,
		  locked_until = CASE WHEN failed_logins + 1 >= $3 THEN now() + make_interval(secs => $4) ELSE locked_until END
		WHERE id = $1 AND shop_id = $2 RETURNING failed_logins`
	if mfa {
		q = `UPDATE staff SET failed_mfa = failed_mfa + 1,
		  locked_until = CASE WHEN failed_mfa + 1 >= $3 THEN now() + make_interval(secs => $4) ELSE locked_until END
		WHERE id = $1 AND shop_id = $2 RETURNING failed_mfa`
	}
	var n int
	err := tx.QueryRowContext(ctx, q, staffID, shopID, after, lock.Seconds()).Scan(&n)
	return n, err
}

// resetFailedLogins: thik password e 0; PBKDF2 er majhe lock hoye gele (parallel bhul) false.
func resetFailedLogins(ctx context.Context, tx *sql.Tx, shopID, staffID int64) (bool, error) {
	return affected(tx.ExecContext(ctx, `UPDATE staff SET failed_logins = 0
		WHERE id = $1 AND shop_id = $2 AND is_active AND (locked_until IS NULL OR locked_until <= now())`, staffID, shopID))
}

func insertStaffSession(ctx context.Context, tx *sql.Tx, shopID, staffID int64, hash []byte, stage, ip, ua string, ttl time.Duration) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, stage, ip, user_agent, expires_at)
		VALUES ($1, $2, 'staff', $3, $4, $5, $6, now() + make_interval(secs => $7))`,
		shopID, hash, staffID, stage, ip, ua, ttl.Seconds())
	return err
}

type mfaRow struct {
	id, shopID, staffID int64
	expired, mustChange bool
	locked              time.Duration
	totpCT, totpNonce   []byte
	keyVersion          int
}

// mfaSession: temp_token er mfa_pending session ar tar active staff. Admin request e shop context nai, shop
// ashe session er row theke (3.2 Niyom 3); token_hash unique. Expire holeo row ashe (session_expired).
func mfaSession(ctx context.Context, d *sql.DB, hash []byte) (mfaRow, error) {
	var r mfaRow
	var secs float64
	err := d.QueryRowContext(ctx, `SELECT s.id, s.shop_id, s.staff_id, s.expires_at <= now(), st.must_change_password,
		  coalesce(greatest(extract(epoch FROM st.locked_until - now()), 0), 0),
		  st.totp_secret_enc, st.totp_nonce, coalesce(st.totp_key_version, 0)
		FROM sessions s JOIN staff st ON st.shop_id = s.shop_id AND st.id = s.staff_id AND st.is_active
		WHERE s.token_hash = $1 AND s.subject_type = 'staff' AND s.stage = 'mfa_pending' AND s.revoked_at IS NULL`,
		hash).Scan(&r.id, &r.shopID, &r.staffID, &r.expired, &r.mustChange, &secs, &r.totpCT, &r.totpNonce, &r.keyVersion)
	r.locked = time.Duration(secs * float64(time.Second))
	return r, err
}

// countMFAAttempt (8.3 step 2): autocommit e conditional, tai parallel e-o ek temp_token e 5 er beshi na.
func countMFAAttempt(ctx context.Context, d *sql.DB, shopID, id int64) (bool, error) {
	return affected(d.ExecContext(ctx, `UPDATE sessions SET mfa_attempts = mfa_attempts + 1
		WHERE id = $1 AND shop_id = $2 AND stage = 'mfa_pending' AND mfa_attempts < 5
		  AND revoked_at IS NULL AND expires_at > now()`, id, shopID))
}

// useTOTPStep (8.3 step 2 v1.5): step totp_last_step er beshi hole-i; ekoi code (parallel o) ekbar.
func useTOTPStep(ctx context.Context, tx *sql.Tx, shopID, staffID, step int64) (bool, error) {
	return affected(tx.ExecContext(ctx, `UPDATE staff SET totp_last_step = $3
		WHERE id = $1 AND shop_id = $2 AND (totp_last_step IS NULL OR totp_last_step < $3)`, staffID, shopID, step))
}

// useRecoveryCode (8.3 step 3): code ekbar (used_at conditional).
func useRecoveryCode(ctx context.Context, tx *sql.Tx, shopID, staffID int64, hash []byte) (bool, error) {
	return affected(tx.ExecContext(ctx, `UPDATE staff_recovery_codes SET used_at = now()
		WHERE shop_id = $1 AND staff_id = $2 AND code_hash = $3 AND used_at IS NULL`, shopID, staffID, hash))
}

// activateSession (8.3 step 3, 4): from stage er (mfa_pending ba setup_pending) row-i notun stage, notun token_hash.
func activateSession(ctx context.Context, tx *sql.Tx, shopID, id int64, from string, hash []byte, stage string, ttl time.Duration) (bool, error) {
	return affected(tx.ExecContext(ctx, `UPDATE sessions SET stage = $4, token_hash = $5,
		  expires_at = now() + make_interval(secs => $6), last_used_at = now()
		WHERE id = $1 AND shop_id = $2 AND stage = $3 AND revoked_at IS NULL AND expires_at > now()`,
		id, shopID, from, stage, hash, ttl.Seconds()))
}

func mfaSucceeded(ctx context.Context, tx *sql.Tx, shopID, staffID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE staff SET failed_mfa = 0, last_login_at = now() WHERE id = $1 AND shop_id = $2`,
		staffID, shopID)
	return err
}

type audit struct {
	shopID, staffID int64 // staffID 0 = ochena (NULL)
	entity          int64 // entity_id; 0 = staffID (nijer kaj)
	action, ip      string
	after           map[string]any
}

// insertAudit (8.3 step 4): audit_logs e ekta row (entity staff).
func insertAudit(ctx context.Context, e jobs.Execer, a audit) error {
	after, err := json.Marshal(a.after)
	if err != nil {
		return err
	}
	staff := sql.NullInt64{Int64: a.staffID, Valid: a.staffID != 0}
	entity := staff
	if a.entity != 0 {
		entity = sql.NullInt64{Int64: a.entity, Valid: true}
	}
	_, err = e.ExecContext(ctx, `INSERT INTO audit_logs (shop_id, staff_id, action, entity_type, entity_id, after, ip)
		VALUES ($1, $2, $3, 'staff', $6, $4, NULLIF($5, ''))`, a.shopID, staff, a.action, after, a.ip, entity)
	return err
}

func affected(res sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

type staffSessionRow struct {
	staff          httpx.Staff
	expired, stale bool
}

// staffSessionByHash: __Host-asid er session (active ba setup_pending, revoke na) ar tar active staff. Shop
// session er row theke (3.2 Niyom 3), token_hash unique. Expire holeo row ashe (session_expired).
func staffSessionByHash(ctx context.Context, d *sql.DB, hash []byte) (staffSessionRow, error) {
	var r staffSessionRow
	var vendor sql.NullInt64
	err := d.QueryRowContext(ctx, `SELECT s.id, s.shop_id, st.id, st.vendor_id, st.role, s.stage = 'setup_pending',
		  s.expires_at <= now(), s.last_used_at < now() - interval '1 hour'
		FROM sessions s JOIN staff st ON st.shop_id = s.shop_id AND st.id = s.staff_id AND st.is_active
		WHERE s.token_hash = $1 AND s.subject_type = 'staff' AND s.stage IN ('active', 'setup_pending')
		  AND s.revoked_at IS NULL`, hash).Scan(&r.staff.SessionID, &r.staff.ShopID, &r.staff.ID, &vendor, &r.staff.Role,
		&r.staff.Setup, &r.expired, &r.stale)
	if vendor.Valid {
		r.staff.VendorID = &vendor.Int64
	}
	return r, err
}

func staffPasswordHash(ctx context.Context, d *sql.DB, shopID, staffID int64) (string, error) {
	var h string
	err := d.QueryRowContext(ctx, `SELECT password_hash FROM staff WHERE id = $1 AND shop_id = $2`, staffID, shopID).Scan(&h)
	return h, err
}

// setPassword (8.3 step 4): notun hash, must_change_password false.
func setPassword(ctx context.Context, tx *sql.Tx, shopID, staffID int64, hash string) error {
	_, err := tx.ExecContext(ctx, `UPDATE staff SET password_hash = $3, must_change_password = false, password_changed_at = now()
		WHERE id = $1 AND shop_id = $2`, staffID, shopID, hash)
	return err
}

// revokeOtherSessions (8.3 step 4): password bodlale ei session chhara staff er shob session (mfa_pending o).
func revokeOtherSessions(ctx context.Context, tx *sql.Tx, shopID, staffID, keep int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = now()
		WHERE shop_id = $1 AND staff_id = $2 AND id <> $3 AND revoked_at IS NULL`, shopID, staffID, keep)
	return err
}

type totpRow struct {
	mustChange      bool
	slug, email     string
	totpCT, totpNon []byte
	keyVersion      int
}

// staffTOTP: setup/enable er jonno staff er obostha, FOR UPDATE: enable er majhe setup secret na bodlay.
func staffTOTP(ctx context.Context, tx *sql.Tx, shopID, staffID int64) (totpRow, error) {
	var r totpRow
	err := tx.QueryRowContext(ctx, `SELECT st.must_change_password, sh.slug, st.email, st.totp_secret_enc, st.totp_nonce,
		  coalesce(st.totp_key_version, 0)
		FROM staff st JOIN shops sh ON sh.id = st.shop_id WHERE st.id = $1 AND st.shop_id = $2 FOR UPDATE OF st`,
		staffID, shopID).Scan(&r.mustChange, &r.slug, &r.email, &r.totpCT, &r.totpNon, &r.keyVersion)
	return r, err
}

// setTOTPSecret (8.3 step 6): enable er age-o purono secret replace.
func setTOTPSecret(ctx context.Context, tx *sql.Tx, shopID, staffID int64, ct, nonce []byte, keyVersion int) error {
	_, err := tx.ExecContext(ctx, `UPDATE staff SET totp_secret_enc = $3, totp_nonce = $4, totp_key_version = $5
		WHERE id = $1 AND shop_id = $2`, staffID, shopID, ct, nonce, keyVersion)
	return err
}

// replaceRecoveryCodes (8.3 step 6): enable e purono shob muche notun 10 ta (hash).
func replaceRecoveryCodes(ctx context.Context, tx *sql.Tx, shopID, staffID int64, hashes [][]byte) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM staff_recovery_codes WHERE shop_id = $1 AND staff_id = $2`, shopID, staffID); err != nil {
		return err
	}
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO staff_recovery_codes (shop_id, staff_id, code_hash) VALUES ($1, $2, $3)`,
			shopID, staffID, h); err != nil {
			return err
		}
	}
	return nil
}

// StaffProfile: GET /api/admin/me (7.7); id na (customer User er moto).
type StaffProfile struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	VendorID *int64 `json:"vendor_id"`
}

func staffProfile(ctx context.Context, d *sql.DB, shopID, staffID int64) (StaffProfile, error) {
	var p StaffProfile
	var vendor sql.NullInt64
	err := d.QueryRowContext(ctx, `SELECT name, email, role, vendor_id FROM staff WHERE id = $1 AND shop_id = $2 AND is_active`,
		staffID, shopID).Scan(&p.Name, &p.Email, &p.Role, &vendor)
	if vendor.Valid {
		p.VendorID = &vendor.Int64
	}
	return p, err
}

// staffForReset: slug er shop ar tar staff (email boro/choto haat mane na), staff row FOR UPDATE (parallel
// totp/enable er sathe na mishe). Shop na thakle sql.ErrNoRows; staff na thakle staffID 0.
func staffForReset(ctx context.Context, tx *sql.Tx, slug, email string) (shopID, staffID int64, err error) {
	if err = tx.QueryRowContext(ctx, `SELECT id FROM shops WHERE slug = $1`, slug).Scan(&shopID); err != nil {
		return 0, 0, err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM staff WHERE shop_id = $1 AND lower(email) = lower($2) FOR UPDATE`,
		shopID, email).Scan(&staffID)
	if errors.Is(err, sql.ErrNoRows) {
		return shopID, 0, nil
	}
	return shopID, staffID, err
}

// clearStaff2FA (8.3 item 6): TOTP ar recovery code muche, shob session revoke; porer login setup_pending.
func clearStaff2FA(ctx context.Context, tx *sql.Tx, shopID, staffID int64) error {
	for _, q := range []string{
		`UPDATE staff SET totp_secret_enc = NULL, totp_nonce = NULL, totp_key_version = NULL, totp_last_step = NULL,
		  failed_mfa = 0 WHERE id = $2 AND shop_id = $1`,
		`DELETE FROM staff_recovery_codes WHERE shop_id = $1 AND staff_id = $2`,
		`UPDATE sessions SET revoked_at = now() WHERE shop_id = $1 AND staff_id = $2 AND revoked_at IS NULL`,
	} {
		if _, err := tx.ExecContext(ctx, q, shopID, staffID); err != nil {
			return err
		}
	}
	return nil
}
