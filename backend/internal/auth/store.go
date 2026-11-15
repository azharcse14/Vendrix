package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// lockPhone: ek shop er ek phone er OTP send ek ek kore (transaction shesh e chhare), jate parallel send e
// count ar insert er majhe onno send dhuke limit par na kore (M4 step 4).
func lockPhone(ctx context.Context, tx *sql.Tx, shopID int64, phone string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('otp:' || $1::bigint || ':' || $2, 0))`, shopID, phone)
	return err
}

func isBlocked(ctx context.Context, tx *sql.Tx, shopID int64, phone string) (bool, error) {
	var ok bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM blocked_phones WHERE shop_id = $1 AND phone = $2)`,
		shopID, phone).Scan(&ok)
	return ok, err
}

// phoneLock (7.8 v1.19, Q3 (a)): shesh 24 ghontay oi phone er bhul = attempts, consumed row e ek kom (shesh
// attempt thik). Bhul >= lockWrong hole lock; khulbe jokhon purono row gulo window theke ber hoye baki bhul
// lockWrong er kom (rolling). Lock na hole ok false.
func phoneLock(ctx context.Context, tx *sql.Tx, shopID int64, phone string, lockWrong int) (time.Duration, bool, error) {
	var secs sql.NullFloat64
	err := tx.QueryRowContext(ctx, `SELECT extract(epoch FROM min(created_at) + interval '24 hours' - now())
		FROM (SELECT created_at, sum(wrong) OVER () AS total, sum(wrong) OVER (ORDER BY created_at, id) AS upto
		      FROM (SELECT id, created_at, greatest(attempts - (consumed_at IS NOT NULL)::int, 0) AS wrong
		            FROM otp_codes WHERE shop_id = $1 AND phone = $2 AND created_at > now() - interval '24 hours') w) x
		WHERE total >= $3 AND total - upto < $3`, shopID, phone, lockWrong).Scan(&secs)
	return seconds(secs), secs.Valid, err
}

// sendLimit (7.8): window e oi phone er (shob purpose) n ta OTP already thakle limit; n tomo shesh ta window
// theke ber howa porjonto opekkha.
func sendLimit(ctx context.Context, tx *sql.Tx, shopID int64, phone string, n int, window time.Duration) (time.Duration, bool, error) {
	var secs sql.NullFloat64
	err := tx.QueryRowContext(ctx, `SELECT extract(epoch FROM created_at + make_interval(secs => $3) - now())
		FROM otp_codes WHERE shop_id = $1 AND phone = $2 AND created_at > now() - make_interval(secs => $3)
		ORDER BY created_at DESC OFFSET $4 LIMIT 1`, shopID, phone, window.Seconds(), n-1).Scan(&secs)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return seconds(secs), err == nil, err
}

// smsCap (7.8 v1.7, Q10 v): shop er timezone er ajker din e OTP (shob phone) >= published
// checkout.sms_daily_cap (na thakle 500) hole limit, shop er rat 12 ta porjonto.
// ponytail: lock phone er, shop er na; parallel e alada phone er send cap ek-dui ta par korte pare.
func smsCap(ctx context.Context, tx *sql.Tx, shopID int64) (time.Duration, bool, error) {
	var hit bool
	var secs float64
	err := tx.QueryRowContext(ctx, `WITH s AS (
		  SELECT date_trunc('day', now() AT TIME ZONE timezone) AT TIME ZONE timezone AS day_start,
		    (date_trunc('day', now() AT TIME ZONE timezone) + interval '1 day') AT TIME ZONE timezone AS day_end
		  FROM shops WHERE id = $1)
		SELECT (SELECT count(*) FROM otp_codes o, s WHERE o.shop_id = $1 AND o.created_at >= s.day_start)
		    >= coalesce((SELECT (published #>> '{checkout,sms_daily_cap}')::int FROM shop_settings WHERE shop_id = $1), 500),
		  extract(epoch FROM day_end - now())
		FROM s`, shopID).Scan(&hit, &secs)
	return time.Duration(secs * float64(time.Second)), hit, err
}

func insertOTP(ctx context.Context, tx *sql.Tx, shopID int64, phone, purpose string, hash []byte, ttl time.Duration) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at)
		VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5)) RETURNING id`,
		shopID, phone, purpose, hash, ttl.Seconds()).Scan(&id)
	return id, err
}

// otpPhone: OTP job er phone (shop shoho, Rule 3).
func otpPhone(ctx context.Context, d *sql.DB, shopID, otpID int64) (string, error) {
	var phone string
	err := d.QueryRowContext(ctx, `SELECT phone FROM otp_codes WHERE id = $1 AND shop_id = $2`, otpID, shopID).Scan(&phone)
	return phone, err
}

func seconds(s sql.NullFloat64) time.Duration {
	return time.Duration(s.Float64 * float64(time.Second))
}

// latestOTP (8.2 verify step 1): oi phone ar purpose er shesh, use na hoya, expire na hoya OTP.
func latestOTP(ctx context.Context, d *sql.DB, shopID int64, phone, purpose string) (int64, error) {
	var id int64
	err := d.QueryRowContext(ctx, `SELECT id FROM otp_codes
		WHERE shop_id = $1 AND phone = $2 AND purpose = $3 AND consumed_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC, id DESC LIMIT 1`, shopID, phone, purpose).Scan(&id)
	return id, err
}

// countAttempt (8.2 verify step 2): autocommit e (verify er transaction er baire), jate porer bhul ba
// ROLLBACK eo gona thake; conditional, tai parallel e-o 5 er beshi na. Kichu na ele ok false.
func countAttempt(ctx context.Context, d *sql.DB, shopID, id int64) ([]byte, bool, error) {
	var hash []byte
	err := d.QueryRowContext(ctx, `UPDATE otp_codes SET attempts = attempts + 1
		WHERE id = $1 AND shop_id = $2 AND attempts < 5 AND consumed_at IS NULL AND expires_at > now()
		RETURNING code_hash`, id, shopID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return hash, err == nil, err
}

func attemptsUsed(ctx context.Context, d *sql.DB, shopID, id int64) (bool, error) {
	var used bool
	err := d.QueryRowContext(ctx, `SELECT attempts >= 5 FROM otp_codes WHERE id = $1 AND shop_id = $2`, id, shopID).Scan(&used)
	return used, err
}

// consumeOTP (8.2 verify step 3 v1.7): conditional, tai ekoi code e duita verify duita session pay na.
func consumeOTP(ctx context.Context, tx *sql.Tx, shopID, id int64) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE otp_codes SET consumed_at = now()
		WHERE id = $1 AND shop_id = $2 AND consumed_at IS NULL`, id, shopID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func scanUser(row *sql.Row) (int64, User, error) {
	var id int64
	var u User
	err := row.Scan(&id, &u.Phone, &u.Name, &u.Email, &u.Locale)
	return id, u, err
}

// upsertUser (8.2 verify step 4): (shop_id, phone) e user; notun hole locale request theke, purono user er
// kichu bodlay na. Delete kora account er phone NULL, tai conflict hoy na, notun account.
func upsertUser(ctx context.Context, tx *sql.Tx, shopID int64, phone, locale string) (int64, User, error) {
	return scanUser(tx.QueryRowContext(ctx, `INSERT INTO users (shop_id, phone, locale) VALUES ($1, $2, $3)
		ON CONFLICT (shop_id, phone) DO UPDATE SET phone = EXCLUDED.phone RETURNING id, phone, name, email, locale`, shopID, phone, locale))
}

// linkGuestOrders (8.2 verify step 5): shudhu OTP verify er por; onno user er order chhoy na.
func linkGuestOrders(ctx context.Context, tx *sql.Tx, shopID, userID int64, phone string) error {
	_, err := tx.ExecContext(ctx, `UPDATE orders SET user_id = $1 WHERE shop_id = $2 AND customer_phone = $3 AND user_id IS NULL`,
		userID, shopID, phone)
	return err
}

func insertSession(ctx context.Context, tx *sql.Tx, shopID, userID int64, hash []byte, ip, ua string, ttl time.Duration) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, ip, user_agent, expires_at)
		VALUES ($1, $2, 'customer', $3, $4, $5, now() + make_interval(secs => $6))`, shopID, hash, userID, ip, ua, ttl.Seconds())
	return err
}

type sessionRow struct {
	id, userID     int64
	expired, stale bool
}

// sessionByHash: ei shop er active customer session (user delete na); expire holeo row ashe (session_expired).
func sessionByHash(ctx context.Context, d *sql.DB, shopID int64, hash []byte) (sessionRow, error) {
	var s sessionRow
	err := d.QueryRowContext(ctx, `SELECT s.id, s.user_id, s.expires_at <= now(), s.last_used_at < now() - interval '1 hour'
		FROM sessions s JOIN users u ON u.shop_id = s.shop_id AND u.id = s.user_id AND u.deleted_at IS NULL
		WHERE s.token_hash = $1 AND s.shop_id = $2 AND s.subject_type = 'customer' AND s.stage = 'active'
		  AND s.revoked_at IS NULL`, hash, shopID).Scan(&s.id, &s.userID, &s.expired, &s.stale)
	return s, err
}

func touchSession(ctx context.Context, d *sql.DB, shopID, id int64) error {
	_, err := d.ExecContext(ctx, `UPDATE sessions SET last_used_at = now()
		WHERE id = $1 AND shop_id = $2 AND last_used_at < now() - interval '1 hour'`, id, shopID)
	return err
}

func revokeSession(ctx context.Context, d *sql.DB, shopID, id int64) error {
	_, err := d.ExecContext(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND shop_id = $2 AND revoked_at IS NULL`, id, shopID)
	return err
}

func getUser(ctx context.Context, d *sql.DB, shopID, userID int64) (User, error) {
	_, u, err := scanUser(d.QueryRowContext(ctx, `SELECT id, phone, name, email, locale FROM users
		WHERE shop_id = $1 AND id = $2 AND deleted_at IS NULL`, shopID, userID))
	return u, err
}

// updateUser (7.3): nil field bodlay na; name/email "" hole NULL.
func updateUser(ctx context.Context, d *sql.DB, shopID, userID int64, p UserPatch) (User, error) {
	_, u, err := scanUser(d.QueryRowContext(ctx, `UPDATE users SET
		  name = CASE WHEN $3 THEN NULLIF($4, '') ELSE name END,
		  email = CASE WHEN $5 THEN NULLIF($6, '') ELSE email END,
		  locale = coalesce(NULLIF($7, ''), locale)
		WHERE shop_id = $1 AND id = $2 AND deleted_at IS NULL RETURNING id, phone, name, email, locale`,
		shopID, userID, p.Name != nil, deref(p.Name), p.Email != nil, deref(p.Email), deref(p.Locale)))
	return u, err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
