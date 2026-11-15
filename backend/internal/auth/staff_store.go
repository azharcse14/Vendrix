package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

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

// activateSession (8.3 step 3): mfa_pending row-i active/setup_pending, notun token_hash.
func activateSession(ctx context.Context, tx *sql.Tx, shopID, id int64, hash []byte, stage string, ttl time.Duration) (bool, error) {
	return affected(tx.ExecContext(ctx, `UPDATE sessions SET stage = $3, token_hash = $4,
		  expires_at = now() + make_interval(secs => $5), last_used_at = now()
		WHERE id = $1 AND shop_id = $2 AND stage = 'mfa_pending' AND revoked_at IS NULL AND expires_at > now()`,
		id, shopID, stage, hash, ttl.Seconds()))
}

func mfaSucceeded(ctx context.Context, tx *sql.Tx, shopID, staffID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE staff SET failed_mfa = 0, last_login_at = now() WHERE id = $1 AND shop_id = $2`,
		staffID, shopID)
	return err
}

type audit struct {
	shopID, staffID int64 // staffID 0 = ochena (NULL)
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
	_, err = e.ExecContext(ctx, `INSERT INTO audit_logs (shop_id, staff_id, action, entity_type, entity_id, after, ip)
		VALUES ($1, $2, $3, 'staff', $2, $4, NULLIF($5, ''))`, a.shopID, staff, a.action, after, a.ip)
	return err
}

func affected(res sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
