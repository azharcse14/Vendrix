package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/jobs"
)

// Staff login er niyom (8.3, 7.8).
const (
	mfaTTL         = 5 * time.Minute  // temp_token (mfa_pending)
	setupTTL       = 15 * time.Minute // setup_pending, cookie Max-Age 900
	adminTTL       = 12 * time.Hour   // active, cookie Max-Age 43200
	loginLockAfter = 5                // eto bhul er por protiti bhul e soft lock
	loginLock      = time.Minute
	mfaLockAfter   = 10 // failed_mfa, temp_token jure
	mfaLock        = 30 * time.Minute
)

var (
	errBadCredentials = errors.New("bad credentials") // 401 invalid_credentials
	errMFAInvalid     = errors.New("mfa invalid")     // 400 mfa_invalid
	errMFAAttempts    = errors.New("mfa 5 attempt")   // 400 mfa_too_many_attempts
)

type loginInput struct {
	Shop, Email, Password string // Shop = slug
	IP, UserAgent         string
}

// staffLogin (8.3 step 1, 2, 4): shop (slug, closed o, Q7) ar email e staff; ochena/inactive e dummy hash er
// tulona; PBKDF2 er age failed_logins er reservation (5 er por 1 minute lock, 10 ar 50 e alert), lock e-o dummy
// PBKDF2 ar 401 (v1.24: lock dekhe email chena na jay); thik hole 0, purono temp_token revoke ar session: TOTP
// thakle mfa_pending (temp_token), na hole setup_pending. Shob login audit e.
func staffLogin(ctx context.Context, d *sql.DB, in loginInput) (token, stage string, err error) {
	st, err := staffForLogin(ctx, d, in.Shop, in.Email)
	if errors.Is(err, sql.ErrNoRows) { // ochena shop: audit er shop nai
		_, err := checkPasswordSem(ctx, dummyHash, in.Password)
		return "", "", cmpErr(err, errBadCredentials)
	}
	if err != nil {
		return "", "", err
	}
	if st.id == 0 || !st.active {
		if _, err := checkPasswordSem(ctx, dummyHash, in.Password); err != nil {
			return "", "", err
		}
		a := audit{shopID: st.shopID, staffID: st.id, action: "staff.login_failed", ip: in.IP,
			after: map[string]any{"reason": "inactive"}}
		if st.id == 0 { // email na: keu email er ghore password likhte pare (12.2 v1.24)
			_, domain, _ := strings.Cut(strings.ToLower(in.Email), "@")
			a.after = map[string]any{"reason": "unknown_email", "email_domain": clip(domain, 254)}
		}
		return "", "", cmpErr(insertAudit(ctx, d, a), errBadCredentials)
	}
	failed := audit{shopID: st.shopID, staffID: st.id, action: "staff.login_failed", ip: in.IP}
	n, _, err := reserveFailed(ctx, d, st.shopID, st.id, false, loginLockAfter, loginLock)
	if err != nil {
		return "", "", err
	}
	if n == 0 { // lock: bhul password er moto shomoy ar response
		if _, err := checkPasswordSem(ctx, dummyHash, in.Password); err != nil {
			return "", "", err
		}
		failed.after = map[string]any{"reason": "locked"}
		return "", "", cmpErr(insertAudit(ctx, d, failed), errBadCredentials)
	}
	ok, err := checkPasswordSem(ctx, st.hash, in.Password)
	if err != nil {
		return "", "", err
	}
	if !ok {
		failed.after = map[string]any{"reason": "password"}
		return "", "", cmpErr(passwordFailed(ctx, d, failed, n), errBadCredentials)
	}
	stage, ttl := "setup_pending", setupTTL
	if st.hasTOTP { // TOTP thakle age TOTP, must_change_password tar pore (8.3 step 4)
		stage, ttl = "mfa_pending", mfaTTL
	}
	token, th := newSessionToken()
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		if err := resetFailedLogins(ctx, tx, st.shopID, st.id, mfaLockAfter); err != nil {
			return err
		}
		if err := revokeMFAPending(ctx, tx, st.shopID, st.id); err != nil {
			return err
		}
		if err := insertStaffSession(ctx, tx, st.shopID, st.id, th, stage, in.IP, in.UserAgent, ttl); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit{shopID: st.shopID, staffID: st.id, action: "staff.login", ip: in.IP,
			after: map[string]any{"stage": stage}})
	})
	return token, stage, err
}

// passwordFailed: bhul password (gona reservation e hoye geche, n): 10 ar 50 e alert, audit.
func passwordFailed(ctx context.Context, d *sql.DB, a audit, n int) error {
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		if n == 10 || n == 50 {
			if err := securityAlert(ctx, tx, a.shopID, a.staffID, "staff_login_failed", n); err != nil {
				return err
			}
		}
		return insertAudit(ctx, tx, a)
	})
}

type mfaInput struct {
	Token, Code, IP string
	Recovery        bool // recovery code, na hole TOTP
}

// verifyMFA (8.3 step 2, 3): temp_token (mfa_pending, 5 minute) → mfa_attempts (5, autocommit e conditional)
// → staff e failed_mfa er reservation (v1.24; lock e 429, parallel temp_token e-o 10 er beshi guess na) → TOTP (±1 step, totp_last_step er conditional UPDATE) ba recovery code (used_at
// conditional). Thik hole oi row-i active (12 ghonta), must_change_password thakle setup_pending (15 minute);
// token notun, karon temp_token JS dekheche; failed_mfa 0. Bhul e audit, 10 e alert ar 429.
func verifyMFA(ctx context.Context, d *sql.DB, totpKey []byte, recoverySecret string, in mfaInput) (token string, setup bool, err error) {
	s, err := mfaSession(ctx, d, tokenHash(in.Token))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, httpx.ErrNoSession
	case err != nil:
		return "", false, err
	case s.expired:
		return "", false, httpx.ErrSessionExpired
	}
	if ok, err := countMFAAttempt(ctx, d, s.shopID, s.id); err != nil || !ok {
		return "", false, cmpErr(err, errMFAAttempts)
	}
	n, locked, err := reserveFailed(ctx, d, s.shopID, s.staffID, true, mfaLockAfter, mfaLock)
	if err != nil {
		return "", false, err
	}
	if n == 0 {
		return "", false, limitError{locked}
	}
	method := "totp"
	var step int64
	var codeHash []byte
	if in.Recovery {
		method, codeHash = "recovery", HashRecoveryCode(recoverySecret, s.staffID, in.Code)
	} else {
		secret, err := OpenTOTPSecret(totpKey, s.shopID, s.staffID, s.keyVersion, s.totpCT, s.totpNonce)
		if err != nil {
			return "", false, err
		}
		var ok bool
		if step, ok = CheckTOTP(secret, in.Code, time.Now()); !ok {
			return "", false, mfaFailed(ctx, d, s, method, in.IP, n)
		}
	}
	stage, ttl := "active", adminTTL
	if s.mustChange {
		stage, ttl = "setup_pending", setupTTL
	}
	token, th := newSessionToken()
	used := false
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		var ok bool
		var err error
		if in.Recovery {
			ok, err = useRecoveryCode(ctx, tx, s.shopID, s.staffID, codeHash)
		} else {
			ok, err = useTOTPStep(ctx, tx, s.shopID, s.staffID, step)
		}
		if used = ok; err != nil || !ok {
			return err
		}
		if ok, err := activateSession(ctx, tx, s.shopID, s.id, "mfa_pending", th, stage, ttl); err != nil || !ok {
			return cmpErr(err, httpx.ErrNoSession)
		}
		if err := mfaSucceeded(ctx, tx, s.shopID, s.staffID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit{shopID: s.shopID, staffID: s.staffID, action: "staff.login_mfa", ip: in.IP,
			after: map[string]any{"method": method, "stage": stage}})
	})
	if err != nil {
		return "", false, err
	}
	if !used { // replay ba bhul/use kora recovery code
		return "", false, mfaFailed(ctx, d, s, method, in.IP, n)
	}
	return token, stage == "setup_pending", nil
}

// mfaFailed: bhul code (failed_mfa reservation e gona hoye geche, n); 10 ba beshi hole lock ar 429, 10 e alert.
func mfaFailed(ctx context.Context, d *sql.DB, s mfaRow, method, ip string, n int) error {
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		if n == mfaLockAfter {
			if err := securityAlert(ctx, tx, s.shopID, s.staffID, "staff_mfa_lock", n); err != nil {
				return err
			}
		}
		return insertAudit(ctx, tx, audit{shopID: s.shopID, staffID: s.staffID, action: "staff.login_mfa_failed", ip: ip,
			after: map[string]any{"method": method}})
	})
	switch {
	case err != nil:
		return err
	case n >= mfaLockAfter:
		return limitError{mfaLock}
	}
	return errMFAInvalid
}

// securityAlert (Q9, 8.17 v1.24): alert.security job, dedupe_key e shop er timezone er din: ek dine ekoi
// staff er ekoi event (login e 10 ba 50) ekbar, thik login e gona 0 hoye abar 10 chhuleo.
func securityAlert(ctx context.Context, tx *sql.Tx, shopID, staffID int64, event string, count int) error {
	day, err := shopDay(ctx, tx, shopID)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("alert.%s:%d:%d:", event, shopID, staffID)
	if event == "staff_login_failed" {
		key += strconv.Itoa(count) + ":"
	}
	return jobs.Enqueue(ctx, tx, jobs.New{ShopID: shopID, Kind: "alert.security", DedupeKey: key + day,
		Payload: map[string]any{"event": event, "staff_id": staffID, "count": count}})
}

// clip: audit e client er lekha choto rakhe.
func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// totpKeyVersion (8.3 step 5): ekhon ekta TOTP_KEY; rotation e notun version.
const totpKeyVersion = 1

// Password er dairgho (8.3 step 6, Q10 ii): okkhor gone; max PBKDF2 er CPU er jonno.
const (
	minPassword = 12
	maxPassword = 128
)

var (
	errWrongPassword          = errors.New("purono password bhul")      // 400 validation_failed (old)
	errPasswordChangeRequired = errors.New("age password bodlate hobe") // 403 password_change_required
	errTOTPLive               = errors.New("TOTP already live")         // 403 forbidden (bodlano shudhu reset-2fa)
)

// changePassword (8.3 step 4, 6): purono password login er moto failed_logins er reservation diye (v1.24: lock e
// 429, bhul e audit + alert); thik hole notun hash (semaphore e), must_change_password false, gona 0, ei session
// chhara staff er shob session revoke, audit. Dairgho ar "purono na" handler e. v1.25: setup_pending e TOTP live
// ar must_change_password hole (ei session TOTP par koreche) session active, notun token ferot; na hole "".
func changePassword(ctx context.Context, d *sql.DB, st httpx.Staff, old, pw, ip string) (string, error) {
	n, locked, err := reserveFailed(ctx, d, st.ShopID, st.ID, false, loginLockAfter, loginLock)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", limitError{locked}
	}
	hash, err := staffPasswordHash(ctx, d, st.ShopID, st.ID)
	if err != nil {
		return "", err
	}
	if ok, err := checkPasswordSem(ctx, hash, old); err != nil || !ok {
		if err != nil {
			return "", err
		}
		failed := audit{shopID: st.ShopID, staffID: st.ID, action: "staff.password_change_failed", ip: ip}
		return "", cmpErr(passwordFailed(ctx, d, failed, n), errWrongPassword)
	}
	if hash, err = hashPasswordSem(ctx, pw); err != nil {
		return "", err
	}
	var token string
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		r, err := staffTOTP(ctx, tx, st.ShopID, st.ID)
		if err != nil {
			return err
		}
		// TOTP chhara setup_pending e enable er age password lage, ar password onno session revoke kore: tai
		// must_change + live TOTP shudhu TOTP par kora session e (8.3 step 4 v1.25)
		if st.Setup && r.mustChange && r.live {
			tok, th := newSessionToken()
			if ok, err := activateSession(ctx, tx, st.ShopID, st.SessionID, "setup_pending", th, "active", adminTTL); err != nil || !ok {
				return cmpErr(err, httpx.ErrNoSession)
			}
			if err := loggedIn(ctx, tx, st.ShopID, st.ID); err != nil {
				return err
			}
			token = tok
		}
		if err := setPassword(ctx, tx, st.ShopID, st.ID, hash); err != nil {
			return err
		}
		if err := resetFailedLogins(ctx, tx, st.ShopID, st.ID, mfaLockAfter); err != nil {
			return err
		}
		if err := revokeOtherSessions(ctx, tx, st.ShopID, st.ID, st.SessionID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit{shopID: st.ShopID, staffID: st.ID, action: "staff.password_changed", ip: ip})
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// totpSetup (8.3 step 4, 6): password bodlanor por; notun secret AES-GCM e pending e (v1.25: live na, ager
// pending replace; live TOTP thakle 403), ferot base32 ar otpauth URL (issuer shop er slug, label slug:email).
func totpSetup(ctx context.Context, d *sql.DB, key []byte, st httpx.Staff, ip string) (secret, otpauth string, err error) {
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		r, err := staffTOTP(ctx, tx, st.ShopID, st.ID)
		if err != nil {
			return err
		}
		switch {
		case r.mustChange:
			return errPasswordChangeRequired
		case r.live:
			return errTOTPLive
		}
		raw, b32 := NewTOTPSecret()
		ct, nonce, err := SealTOTPSecret(key, st.ShopID, st.ID, totpKeyVersion, raw)
		if err != nil {
			return err
		}
		if err := setPendingTOTP(ctx, tx, st.ShopID, st.ID, ct, nonce, totpKeyVersion); err != nil {
			return err
		}
		secret = b32
		otpauth = (&url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + r.slug + ":" + r.email,
			RawQuery: url.Values{"secret": {b32}, "issuer": {r.slug}, "algorithm": {"SHA1"}, "digits": {"6"},
				"period": {"30"}}.Encode()}).String()
		return insertAudit(ctx, tx, audit{shopID: st.ShopID, staffID: st.ID, action: "staff.totp_setup", ip: ip})
	})
	return secret, otpauth, err
}

// totpEnable (8.3 step 4, 6): pending secret er code (±1 step) thik hole pending → live (v1.25), notun 10
// recovery code (purono muche), setup_pending row-i active 12 ghonta, token notun (setup token JS dekhe na,
// tobu stage bodlale notun, verifyMFA er moto). Bhul code e pending thake ar failed_mfa na: setup session
// password e pawa, TOTP er lock er dorkar nai.
func totpEnable(ctx context.Context, d *sql.DB, key []byte, recoverySecret string, st httpx.Staff, code, ip string) (string, []string, error) {
	codes := NewRecoveryCodes()
	hashes := make([][]byte, len(codes))
	for i, c := range codes {
		hashes[i] = HashRecoveryCode(recoverySecret, st.ID, c)
	}
	token, th := newSessionToken()
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		r, err := staffTOTP(ctx, tx, st.ShopID, st.ID)
		switch {
		case err != nil:
			return err
		case r.mustChange:
			return errPasswordChangeRequired
		case r.live:
			return errTOTPLive
		case r.pendCT == nil:
			return errMFAInvalid
		}
		secret, err := OpenTOTPSecret(key, st.ShopID, st.ID, r.keyVersion, r.pendCT, r.pendNon)
		if err != nil {
			return err
		}
		step, ok := CheckTOTP(secret, code, time.Now())
		if !ok {
			return errMFAInvalid
		}
		if ok, err := livePendingTOTP(ctx, tx, st.ShopID, st.ID, step); err != nil || !ok {
			return cmpErr(err, errMFAInvalid)
		}
		if err := replaceRecoveryCodes(ctx, tx, st.ShopID, st.ID, hashes); err != nil {
			return err
		}
		if ok, err := activateSession(ctx, tx, st.ShopID, st.SessionID, "setup_pending", th, "active", adminTTL); err != nil || !ok {
			return cmpErr(err, httpx.ErrNoSession)
		}
		if err := loggedIn(ctx, tx, st.ShopID, st.ID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit{shopID: st.ShopID, staffID: st.ID, action: "staff.totp_enabled", ip: ip})
	})
	if err != nil {
		return "", nil, err
	}
	return token, codes, nil
}

// reset-2fa er error (CLI spashto bole).
var (
	ErrShopNotFound  = errors.New("ei slug er shop nai")
	ErrStaffNotFound = errors.New("ei shop e ei email er staff nai")
)

// ResetStaff2FA (8.3 item 6, 7.7): owner TOTP ar recovery code duitai harale CLI er ekmatro path. Ek
// transaction e TOTP (pending o), recovery code muche, shob session revoke, audit (actor CLI, staff_id NULL).
func ResetStaff2FA(ctx context.Context, d *sql.DB, slug, email string) error {
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		shopID, staffID, err := staffForReset(ctx, tx, slug, email)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrShopNotFound
		}
		if err != nil {
			return err
		}
		if staffID == 0 {
			return ErrStaffNotFound
		}
		if err := clearStaff2FA(ctx, tx, shopID, staffID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, audit{shopID: shopID, entity: staffID, action: "staff.reset_2fa",
			after: map[string]any{"actor": "cli"}})
	})
}
