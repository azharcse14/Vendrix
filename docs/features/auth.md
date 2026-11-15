# Login (auth)

Design: 8.2, 8.3, 7.3, 7.8, 8.17, 10.11, 12.2 · Milestone: M4 · Code: `backend/internal/auth`

## Ki kore

Customer phone + OTP diye login kore, staff email + password (+ TOTP) diye. Ekhon ache primitive (staff password er hash, OTP er code ar hash, TOTP (RFC 6238) ar tar secret er encryption, recovery code) ar `POST /api/auth/otp/send`: Turnstile, phone er limit ar lock, shop er SMS cap par hole OTP SMS job e jay. OTP verify, session, staff login, TOTP setup ar recovery er endpoint M4 er porer step e ([M4](../milestones/M4.md)).

## Kivabe kaj kore

1. Staff password (`shop create` e owner): `password.go` → `HashPassword` (pbkdf2-sha256, 600000, 16 byte salt); tulona `CheckPassword`.
2. OTP code: `otp.go` → `NewOTPCode`: `crypto/rand` e 0..999999, `%06d` (samne zero thake).
3. OTP hash: `HashOTP(OTP_SECRET, phone, purpose, code)` = `HMAC-SHA256(secret, phone:purpose:code)`; `otp_codes.code_hash` e jabe (step 4). Phone `phone.Normalize` er por E.164.
4. Verify: `CheckOTP` notun hash banay ar `hmac.Equal` (constant time) diye tulona.
5. TOTP: `totp.go` → `TOTPCode(secret, step)` (HMAC-SHA1, dynamic truncation, 6 digit); `CheckTOTP(secret, code, now)` step `now/30` er -1, 0, +1 dekhe, mille sei step dey (replay atkate `staff.totp_last_step` e, step 6).
6. TOTP secret: `NewTOTPSecret` 20 byte random + base32 (padding chhara); DB te `SealTOTPSecret` (`seal.go` → `Seal`, AES-GCM, key `TOTP_KEY`, AAD `shop_id:staff_id:key_version`), pora `OpenTOTPSecret`.
7. OTP send (`POST /api/auth/otp/send`, `cmd/api/router.go` e public limit + client IP te ghontay 10): `handler.go` → `SendOTP` body (`purpose` shudhu `login`), `phone.Normalize`, tarpor `service.go` → `sendOTP`:
   - `Turnstile.Verify` (`turnstile.go`): siteverify e `secret`, `response`, `remoteip`; `success` ar `hostname` = `X-Shop-Host` (`httpx.Shop.Domain`). DB er age.
   - `db.InTx`: `lockPhone` (`pg_advisory_xact_lock`, shop + phone) → `isBlocked` → `phoneLock` (24 ghontar bhul, lock hole `lockAlert` commit kore `429`) → `sendLimit` (3/15 minute, 10/24 ghonta, shob purpose) → `smsCap` (shop er timezone er din, `checkout.sms_daily_cap`) → `insertOTP` (5 minute) → `Seal` (key `otpSMSKey` = HKDF(`OTP_SECRET`), AAD otp id) → `jobs.Enqueue` `sms.send` (`max_attempts` 2).
   - Response `{"sent": true}`; limit/lock/cap e `429 rate_limited` + `Retry-After`.
8. SMS: worker er `notify.SendSMSJob` payload e `otp_id` pele `SMSCode` (phone `otp_codes` theke shop shoho, code decrypt) dake; done hole worker payload `{}` kore ([jobs](jobs.md)).
9. Recovery: `recovery.go` → `NewRecoveryCodes` 10 ta `XXXXX-XXXXX`; `HashRecoveryCode(RECOVERY_SECRET, staff_id, code)` `-`/space bad, boro haat kore `HMAC-SHA256(secret, staff_id:CODE)`. DB te hash diye khoja (PK `staff_id, code_hash`), tai alada tulona function nai.

## Code er map

| File → function | Ki kore |
|---|---|
| `backend/internal/auth/doc.go` | Package comment |
| `backend/internal/auth/password.go` → `HashPassword`, `CheckPassword` | Staff password hash ar tulona |
| `backend/internal/auth/otp.go` → `NewOTPCode` | 6 digit OTP |
| `backend/internal/auth/otp.go` → `HashOTP`, `CheckOTP` | OTP er HMAC ar constant-time tulona |
| `backend/internal/auth/totp.go` → `TOTPCode`, `CheckTOTP` | RFC 6238 code ar ±1 step er check |
| `backend/internal/auth/totp.go` → `NewTOTPSecret`, `SealTOTPSecret`, `OpenTOTPSecret` | Secret banano (base32) ar AES-GCM (AAD shoho) |
| `backend/internal/auth/recovery.go` → `NewRecoveryCodes`, `HashRecoveryCode` | Recovery code ar tar hash |
| `backend/internal/auth/seal.go` → `Seal`, `Open` | AES-GCM + AAD; shop credential ([credentials](credentials.md)), TOTP secret ar OTP SMS payload |
| `backend/internal/auth/handler.go` → `NewHandlers`, `SendOTP` | `POST /api/auth/otp/send` er HTTP, error code |
| `backend/internal/auth/service.go` → `sendOTP`, `lockAlert`, `otpSMSKey`, `SMSCode` | OTP send er niyom, alert, payload er key, job er decrypt |
| `backend/internal/auth/store.go` → `lockPhone`, `isBlocked`, `phoneLock`, `sendLimit`, `smsCap`, `insertOTP`, `otpPhone` | SQL |
| `backend/internal/auth/turnstile.go` → `Turnstile.Verify` | Cloudflare siteverify |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Password: pbkdf2-sha256, 600000, 16 byte salt, format `pbkdf2-sha256$600000$<salt>$<hash>`, constant-time tulona, bhul format e false (8.3) | `HashPassword`, `CheckPassword` | `TestPasswordHash` |
| OTP `crypto/rand`, 6 digit, samne zero shoho (8.2 step 2) | `NewOTPCode` | `TestOTPCode` |
| `code_hash = HMAC-SHA256(OTP_SECRET, phone:purpose:code)`; phone, purpose, code ba secret bodlale mile na; tulona `hmac.Equal` (8.2 step 3, verify step 3) | `HashOTP`, `CheckOTP` | `TestOTPHash` |
| TOTP RFC 6238: HMAC-SHA1, 30 second, 6 digit, ±1 step (±2 na), 6 digit chhara na; secret 20 byte, base32 (8.3 step 2, 6) | `TOTPCode`, `CheckTOTP`, `NewTOTPSecret` | `TestTOTP` (appendix B vector) |
| TOTP secret AES-GCM `TOTP_KEY`, AAD `shop_id:staff_id:key_version`; onno staff/shop/version/key e decrypt na (8.3 step 5) | `SealTOTPSecret`, `OpenTOTPSecret`, `Seal`, `Open` | `TestTOTPSecretAAD` |
| OTP send: phone e 15 minute e 3, 24 ghontay 10 (shob purpose, shop alada), parallel e o (advisory lock); blocked phone `403 phone_blocked`; response `{"sent": true}` (8.2, 7.8, Q10 vii) | `sendOTP`, `lockPhone`, `sendLimit`, `isBlocked` | `TestOTPSendLimitsIntegration` |
| Phone lock: 24 ghontay `otp_codes` er bhul (consumed row e ek kom) 10 hole `429` + rolling `Retry-After`, alert ekbar (`dedupe_key` `alert.otp_lock:<shop>:<phone>:<din>`), payload e phone mask (7.8 v1.19, Q3) | `phoneLock`, `lockAlert` | `TestOTPPhoneLockIntegration` |
| Shop er SMS cap: shop er timezone er din e OTP >= `checkout.sms_daily_cap` hole `429`, rat 12 ta porjonto (7.8 v1.7, Rule 5) | `smsCap` | `TestOTPSMSCapIntegration` |
| Turnstile: siteverify e secret, token, remoteip; `success` ar `hostname` = shop er domain; bhul token e `400 turnstile_failed` (12.2, 10.11, 14.2 test 23) | `Turnstile.Verify`, `SendOTP` | `TestTurnstile` |
| OTP SMS payload `{otp_id, nonce, ct}` (AES-GCM, key HKDF(`OTP_SECRET`), AAD otp id), plain code na; `max_attempts` 2; done hole `{}` (8.2 step 4, 8.17, Q1) | `sendOTP`, `SMSCode`, `jobs.finish` | `TestOTPJobPayload` |
| Client IP te ghontay 10 OTP send; website er `X-Client-IP` shudhu secret shoho (7.8, 3.2 Niyom 5, 14.2 test 18) | `cmd/api/router.go` → `newHandler` | `TestOTPSendIPLimit` |
| Recovery: 10 ta, `crypto/rand` 10 okkhor base32, `XXXXX-XXXXX`; hash `HMAC-SHA256(RECOVERY_SECRET, staff_id:code)`, `-`/space/case bad (8.3 step 5, 6) | `NewRecoveryCodes`, `HashRecoveryCode` | `TestRecoveryCode` |

## Data

`staff.password_hash`; `otp_codes.code_hash` (`BYTEA`, 32 byte) — migration `001_init.sql`. `staff.totp_secret_enc`, `totp_nonce`, `totp_key_version` (`staff_totp_check`: tinta ekshathe NULL ba na), `totp_last_step`; `staff_recovery_codes.code_hash`. OTP send `otp_codes` e row (`expires_at` 5 minute) ar `jobs` e `sms.send` (ba lock e `alert.security`) lekhe, `blocked_phones` pore, `shop_settings.published` er `checkout.sms_daily_cap` ar `shops.timezone` pore; index `otp_phone_idx`. TOTP, recovery er table e lekha step 6-7 e.

## Bhul hole ki hoy

- `CheckPassword`: bhul format, iteration 1 er kom ba 10M er beshi, bhul base64 e `false`.
- `CheckOTP`: `nil` ba choto hash, bhul code/phone/purpose/secret e `false`.
- `CheckTOTP`: 6 okkhor chhara, bhul digit, window er baire (±2 step) ba onno secret e `false`.
- `OpenTOTPSecret`: key, shop, staff ba version na mille error (GCM auth fail); `Seal`/`Open` 32/24/16 byte chhara key e error.
- `HashRecoveryCode` kokhono fail kore na; bhul code er hash DB te mile na.
- OTP send: bhul JSON ba `purpose` `400 validation_failed`; bhul phone `400 invalid_phone`; Turnstile fail `400 turnstile_failed`; Cloudflare e jete na parle ba DB error `500 internal` (log e phone mask); blocked `403 phone_blocked`; limit, lock ba cap `429 rate_limited` + `Retry-After`.
- `SMSCode`: onno shop, onno otp id ba onno `OTP_SECRET` e error (job retry, shesh e failed ar alert).
- `NewOTPCode`: `crypto/rand` fail korle panic (Go te rand fail kore na; korle chalano nirapod na).

## Bodlanor age mone rakho

- Hash er input (`phone:purpose:code`) ba secret bodlale DB te thaka chalu OTP (5 minute) ar mile na; `TestOTPHash` er openssl vector o bodlate hobe (8.2 age).
- Phone sob shomoy E.164 e dao; normalize chhara hash alada hoy.
- `OTP_SECRET` env (13.5, `config.Config.OTPSecret`); rotate korle shudhu chalu OTP gulo batil.
- TOTP er AAD format ba `TOTP_KEY` bodlale DB er shob secret decrypt hoy na: notun `totp_key_version` diye re-encrypt (8.3 step 5). `Seal`/`Open` shop credential o use kore (`TestSealOpen`).
- Cloudflare er test secret (`1x...AA`) siteverify e shob shomoy `hostname` `example.com` dey, tai hostname check local e `a.com`/`localhost` er sathe mile na: 16.1 er proshno (local login er age).
- OTP send er limit er number `service.go` er `otpSendLimits`, `otpLockWrong`; bodlate 7.8 age (`design-change`).
- `otpSMSKey` er info ba AAD bodlale queue te thaka OTP job decrypt hoy na (5 minute e mullohin, tai shudhu deploy er muhurter OTP).
- Recovery hash er input (normalize, `staff_id:code`) ba `RECOVERY_SECRET` bodlale shob staff er code batil; `TestRecoveryCode` er vector o bodlao.

## Proshno-uttor

- **Code `string` keno, `int` na?** Samne zero (`012345`) thakte hobe; SMS, hash ar input shob ekoi lekha.
- **TOTP library keno na?** 2.2: stdlib + pgx chhara dependency na; RFC 6238 20 line, appendix B vector diye test.
- **Limit e alada alada error na keno?** Shob `429 rate_limited`: lock ache naki limit, phone er obostha bole na (7.8).
- **Blocked phone `403` keno, `{"sent": true}` na?** M4 er scope e `phone_blocked` (7.1); admin er block customer ke jante hobe.
- **±1 step keno?** Phone ar server er ghori 30 second porjonto agepiche hoy; beshi window e anda korar shujog bare.
- **`==` na `hmac.Equal` keno?** `==` prothom bhul byte e thame; shomoy mepe hash anda kora jay.
