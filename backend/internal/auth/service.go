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
	"vendrix/backend/internal/notify"
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
	errOTPInvalid   = errors.New("otp invalid")   // 400 otp_invalid
	errOTPAttempts  = errors.New("otp 5 attempt") // 400 otp_too_many_attempts
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
	return issueOTP(ctx, d, secret, in)
}

// issueOTP (8.2 step 1-4, Turnstile er por): login ar checkout dui purpose e ekoi limit, cap, lock.
func issueOTP(ctx context.Context, d *sql.DB, secret string, in sendInput) error {
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
			return lockAlert(ctx, tx, secret, in)
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
// dedupe_key e phone er HMAC tag (v1.26): jobs row 30 din thake, plain phone PII.
func lockAlert(ctx context.Context, tx *sql.Tx, secret string, in sendInput) error {
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil {
		loc = time.UTC
	}
	day := time.Now().In(loc).Format(time.DateOnly)
	return jobs.Enqueue(ctx, tx, jobs.New{ShopID: in.ShopID, Kind: "alert.security",
		DedupeKey: "alert.otp_lock:" + strconv.FormatInt(in.ShopID, 10) + ":" + phoneTag(secret, in.Phone) + ":" + day,
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

// SMSCode: notify.SendSMSJob er OTPSource. otp_codes theke phone (shop shoho), customer er locale ar shop er
// sms.otp override, code decrypt (AAD otp id).
func SMSCode(d *sql.DB, secret string) notify.OTPSource {
	return func(ctx context.Context, shopID, otpID int64, nonce, ct []byte) (notify.OTP, error) {
		key, err := otpSMSKey(secret)
		if err != nil {
			return notify.OTP{}, err
		}
		o, err := otpSMS(ctx, d, shopID, otpID)
		if err != nil {
			return notify.OTP{}, err
		}
		code, err := Open(key, strconv.FormatInt(otpID, 10), ct, nonce)
		if err != nil {
			return notify.OTP{}, err
		}
		o.Code = string(code)
		return o, nil
	}
}

// User: /api/auth/otp/verify ar /api/me er profile (7.3). ID dey na (Rule 11 er moto, client er lage na).
type User struct {
	Phone  string  `json:"phone"`
	Name   *string `json:"name"`
	Email  *string `json:"email"`
	Locale string  `json:"locale"`
}

// UserPatch: PATCH /api/me; nil = bodlabe na.
type UserPatch struct {
	Name, Email, Locale *string
}

type verifyInput struct {
	ShopID                int64
	Timezone              string
	Phone, Code           string // Phone E.164
	Locale, IP, UserAgent string // notun user er locale (Accept-Language), session er ip, user_agent
	CartKey               string // X-Cart-Key (8.2 step 6); "" ba bhul format = merge na
}

// CartMerge (8.2 step 6): verify er tx e X-Cart-Key er guest cart merge, notice ferot (7.3 cart_notices).
// Router cart.Merge dey; auth cart import kore na, karon shop → auth, ar cart → catalog er test → shop (cycle).
type CartMerge func(ctx context.Context, tx *sql.Tx, shopID, userID int64, key, locale string) (any, error)

// verifyOTP (8.2 verify step 1-7): lock → shesh OTP → attempt (nijer commit e) → hmac.Equal → ek transaction
// e consume, user upsert, guest order jog, cart merge, session. Bhul code e 24 ghontar bhul 10 hole lock (429) ar alert.
// Merge ei transaction e, tai verify fail hole merge na; notice (item_dropped) response er cart_notices e (7.3).
func verifyOTP(ctx context.Context, d *sql.DB, secret string, merge CartMerge, in verifyInput) (string, User, any, error) {
	id, err := checkCode(ctx, d, secret, in, "login")
	if err != nil {
		return "", User{}, nil, err
	}
	token, th := newSessionToken()
	var u User
	var notices any
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		if ok, err := consumeOTP(ctx, tx, in.ShopID, id); err != nil || !ok {
			return cmpErr(err, errOTPInvalid)
		}
		userID, user, err := upsertUser(ctx, tx, in.ShopID, in.Phone, in.Locale)
		if err != nil {
			return err
		}
		u = user
		if err := linkGuestOrders(ctx, tx, in.ShopID, userID, in.Phone); err != nil {
			return err
		}
		if notices, err = merge(ctx, tx, in.ShopID, userID, in.CartKey, in.Locale); err != nil {
			return err
		}
		return insertSession(ctx, tx, in.ShopID, userID, th, in.IP, in.UserAgent, customerSessionTTL)
	})
	return token, u, notices, err
}

// checkCode (8.2 verify step 1-3, consume chhara): lock → shesh OTP → attempt (nijer commit e) → hmac.Equal.
// Thik hole OTP er id; consume caller er transaction e.
func checkCode(ctx context.Context, d *sql.DB, secret string, in verifyInput, purpose string) (int64, error) {
	if err := phoneLocked(ctx, d, secret, in); err != nil {
		return 0, err
	}
	id, err := latestOTP(ctx, d, in.ShopID, in.Phone, purpose)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errOTPInvalid
	}
	if err != nil {
		return 0, err
	}
	hash, ok, err := countAttempt(ctx, d, in.ShopID, id)
	if err != nil {
		return 0, err
	}
	if !ok {
		if used, err := attemptsUsed(ctx, d, in.ShopID, id); err != nil || used {
			return 0, cmpErr(err, errOTPAttempts)
		}
		return 0, errOTPInvalid // ei majhe expire ba onno request e use
	}
	if !CheckOTP(secret, in.Phone, purpose, in.Code, hash) {
		return 0, cmpErr(phoneLocked(ctx, d, secret, in), errOTPInvalid)
	}
	return id, nil
}

// Checkout OTP (8.6 step 3, 8, 9; 8.19 item 4): purpose checkout, login er ekoi send limit (shob purpose
// mile), SMS cap, phone lock ar attempt. Error: ErrOTPInvalid, ErrOTPAttempts, ErrPhoneBlocked, RetryAfter.
var (
	ErrOTPInvalid   = errOTPInvalid
	ErrOTPAttempts  = errOTPAttempts
	ErrPhoneBlocked = errPhoneBlocked
)

// RetryAfter: 429 rate_limited er error hole koto por; ok false = limit na.
func RetryAfter(err error) (time.Duration, bool) {
	var l limitError
	if errors.As(err, &l) {
		return l.retry, true
	}
	return 0, false
}

// SendCheckoutOTP: Turnstile caller (order) step 3 e dekheche; tarpor login er moto issueOTP.
func SendCheckoutOTP(ctx context.Context, d *sql.DB, secret string, shopID int64, timezone, phone string) error {
	return issueOTP(ctx, d, secret, sendInput{ShopID: shopID, Timezone: timezone, Phone: phone, Purpose: "checkout"})
}

// VerifyCheckoutOTP: code thik hole OTP er id (consume na; ConsumeOTP order er transaction e, 8.6 step 9).
func VerifyCheckoutOTP(ctx context.Context, d *sql.DB, secret string, shopID int64, timezone, phone, code string) (int64, error) {
	return checkCode(ctx, d, secret, verifyInput{ShopID: shopID, Timezone: timezone, Phone: phone, Code: code}, "checkout")
}

// ConsumeOTP (8.6 step 9): conditional; false = onno request e already use (400 otp_invalid).
func ConsumeOTP(ctx context.Context, tx *sql.Tx, shopID, id int64) (bool, error) {
	return consumeOTP(ctx, tx, shopID, id)
}

// phoneLocked (7.8, Q3): 24 ghontay 10 bhul hole limitError, ar alert.security (din e ekbar, lockAlert).
func phoneLocked(ctx context.Context, d *sql.DB, secret string, in verifyInput) error {
	var limit error
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		limit = nil
		retry, locked, err := phoneLock(ctx, tx, in.ShopID, in.Phone, otpLockWrong)
		if err != nil || !locked {
			return err
		}
		limit = limitError{retry}
		return lockAlert(ctx, tx, secret, sendInput{ShopID: in.ShopID, Timezone: in.Timezone, Phone: in.Phone})
	})
	return cmpErr(err, limit)
}
