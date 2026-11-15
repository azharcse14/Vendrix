package auth

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/phone"
)

// OTP send er niyom (8.2, 7.8).
const (
	otpTTL         = 5 * time.Minute
	otpLockWrong   = 10 // 24 ghontay eto bhul e phone lock
	otpMaxAttempts = 2  // sms.send: 1 minute por ekbar (8.17), OTP 5 minute e expire
)

// otpSendLimits (7.8): phone e (shob purpose) 15 minute e 3, 24 ghontay 10.
var otpSendLimits = []struct {
	n      int
	window time.Duration
}{{3, 15 * time.Minute}, {10, 24 * time.Hour}}

var (
	errTurnstile    = errors.New("turnstile failed")
	errPhoneBlocked = errors.New("phone blocked")
)

// limitError: 429 rate_limited, Retry-After shoho.
type limitError struct{ retry time.Duration }

func (e limitError) Error() string { return fmt.Sprintf("rate limited %v", e.retry) }

type sendInput struct {
	ShopID                    int64
	Timezone, Host            string // shop er timezone (alert er din), X-Shop-Host (Turnstile hostname)
	Phone, Purpose, Token, IP string // Phone E.164
}

// sendOTP (8.2 step 1-4): Turnstile → blocked → 24 ghontar lock → phone limit → shop er SMS cap → code,
// otp_codes, sms.send job. Check ar insert ek transaction e phone er advisory lock er pore.
func sendOTP(ctx context.Context, d *sql.DB, secret string, ts Turnstile, in sendInput) error {
	ok, err := ts.Verify(ctx, in.Token, in.IP, in.Host)
	if err != nil {
		return err
	}
	if !ok {
		return errTurnstile
	}
	key, err := otpSMSKey(secret)
	if err != nil {
		return err
	}
	code := NewOTPCode()
	var limit error
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		limit = nil
		if err := lockPhone(ctx, tx, in.ShopID, in.Phone); err != nil {
			return err
		}
		if blocked, err := isBlocked(ctx, tx, in.ShopID, in.Phone); err != nil || blocked {
			return cmpErr(err, errPhoneBlocked)
		}
		retry, locked, err := phoneLock(ctx, tx, in.ShopID, in.Phone, otpLockWrong)
		if err != nil {
			return err
		}
		if locked {
			// alert commit hobe, tai error na diye limit
			limit = limitError{retry}
			return lockAlert(ctx, tx, in)
		}
		for _, l := range otpSendLimits {
			if retry, hit, err := sendLimit(ctx, tx, in.ShopID, in.Phone, l.n, l.window); err != nil || hit {
				return cmpErr(err, limitError{retry})
			}
		}
		if retry, hit, err := smsCap(ctx, tx, in.ShopID); err != nil || hit {
			return cmpErr(err, limitError{retry})
		}
		id, err := insertOTP(ctx, tx, in.ShopID, in.Phone, in.Purpose, HashOTP(secret, in.Phone, in.Purpose, code), otpTTL)
		if err != nil {
			return err
		}
		ct, nonce, err := Seal(key, strconv.FormatInt(id, 10), []byte(code))
		if err != nil {
			return err
		}
		return jobs.Enqueue(ctx, tx, jobs.New{ShopID: in.ShopID, Kind: "sms.send", MaxAttempts: otpMaxAttempts,
			Payload: otpPayload{OTPID: id, Nonce: nonce, CT: ct}})
	})
	if err != nil {
		return err
	}
	return limit
}

// cmpErr: DB er error thakle seta, na hole niyom er error.
func cmpErr(dbErr, rule error) error {
	if dbErr != nil {
		return dbErr
	}
	return rule
}

// lockAlert (7.8, Q9): alert.security, din e ekbar (dedupe_key e shop er timezone er din); payload e phone mask.
func lockAlert(ctx context.Context, tx *sql.Tx, in sendInput) error {
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil {
		loc = time.UTC
	}
	day := time.Now().In(loc).Format(time.DateOnly)
	return jobs.Enqueue(ctx, tx, jobs.New{ShopID: in.ShopID, Kind: "alert.security",
		DedupeKey: "alert.otp_lock:" + strconv.FormatInt(in.ShopID, 10) + ":" + in.Phone + ":" + day,
		Payload:   map[string]string{"event": "otp_phone_lock", "phone": phone.Mask(in.Phone)}})
}

// otpPayload (8.2 step 4 v1.19, Q1 (a)): sms.send e code AES-GCM e, AAD otp_codes.id; plain code WAL/backup e na.
type otpPayload struct {
	OTPID int64  `json:"otp_id"`
	Nonce []byte `json:"nonce"`
	CT    []byte `json:"ct"`
}

// otpSMSKey: OTP_SECRET theke HKDF-SHA256 (notun env na, 13.5); info alada, tai HMAC er sathe key mishe na.
func otpSMSKey(secret string) ([]byte, error) {
	return hkdf.Key(sha256.New, []byte(secret), nil, "vendrix otp sms payload", 32)
}

// SMSCode: notify.SendSMSJob er OTPSource. otp_codes theke phone (shop shoho), code decrypt (AAD otp id).
func SMSCode(d *sql.DB, secret string) func(ctx context.Context, shopID, otpID int64, nonce, ct []byte) (string, string, error) {
	return func(ctx context.Context, shopID, otpID int64, nonce, ct []byte) (string, string, error) {
		key, err := otpSMSKey(secret)
		if err != nil {
			return "", "", err
		}
		phone, err := otpPhone(ctx, d, shopID, otpID)
		if err != nil {
			return "", "", err
		}
		code, err := Open(key, strconv.FormatInt(otpID, 10), ct, nonce)
		if err != nil {
			return "", "", err
		}
		return phone, string(code), nil
	}
}
