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
