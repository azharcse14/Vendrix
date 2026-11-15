# Login (auth)

Design: 8.2, 8.3, 7.3, 7.8, 8.17, 10.11, 12.2 · Milestone: M4 · Code: `backend/internal/auth`

## Ki kore

Customer phone + OTP diye login kore, staff email + password + TOTP diye. `POST /api/auth/otp/send`: Turnstile, phone er limit ar lock, shop er SMS cap par hole OTP SMS job e jay. `POST /api/auth/otp/verify` code mile gele customer account (notun hole banay), oi phone er guest order account e jog ar 30 diner session token dey; `GET`/`PATCH /api/me` profile (naam, email, bhasha), `POST /api/auth/logout`. Staff `POST /api/admin/auth/login` (shop slug, email, password) e TOTP thakle 5 minuter `temp_token` pay, tarpor `/auth/totp` ba `/auth/recovery` e 12 ghontar admin session (`__Host-asid` cookie); TOTP na thakle 15 minuter `setup_pending` session: age password bodlay (`/auth/password`), tarpor `/auth/totp/setup` ar `/auth/totp/enable` (10 recovery code, session active). Admin route `__Host-asid` cookie er session diye chene (`GET /api/admin/me`, `/auth/logout`).

## Kivabe kaj kore

1. Staff password (`shop create` e owner): `password.go` → `HashPassword` (pbkdf2-sha256, 600000, 16 byte salt); tulona `CheckPassword`.
2. OTP code: `otp.go` → `NewOTPCode`: `crypto/rand` e 0..999999, `%06d` (samne zero thake).
3. OTP hash: `HashOTP(OTP_SECRET, phone, purpose, code)` = `HMAC-SHA256(secret, phone:purpose:code)`; `otp_codes.code_hash` e jay (step 7). Phone `phone.Normalize` er por E.164.
4. Verify: `CheckOTP` notun hash banay ar `hmac.Equal` (constant time) diye tulona.
5. TOTP: `totp.go` → `TOTPCode(secret, step)` (HMAC-SHA1, dynamic truncation, 6 digit); `CheckTOTP(secret, code, now)` step `now/30` er -1, 0, +1 dekhe, mille sei step dey (replay atkay `staff.totp_last_step`, step 14).
6. TOTP secret: `NewTOTPSecret` 20 byte random + base32 (padding chhara); DB te `SealTOTPSecret` (`seal.go` → `Seal`, AES-GCM, key `TOTP_KEY`, AAD `shop_id:staff_id:key_version`), pora `OpenTOTPSecret`.
7. OTP send (`POST /api/auth/otp/send`, `cmd/api/router.go` e public limit + client IP te ghontay 10): `handler.go` → `SendOTP` body (`purpose` shudhu `login`), `phone.Normalize`, tarpor `service.go` → `sendOTP`:
   - `Turnstile.Verify` (`turnstile.go`): siteverify e `secret`, `response`, `remoteip`; `success` ar `hostname` = `X-Shop-Host` (`httpx.Shop.Domain`); secret Cloudflare er test secret (`config.TurnstileTestSecret`) hole hostname check na (v1.22). DB er age.
   - `db.InTx`: `lockPhone` (`pg_advisory_xact_lock`, shop + phone) → `isBlocked` → `phoneLock` (24 ghontar bhul, lock hole `lockAlert` commit kore `429`) → `sendLimit` (3/15 minute, 10/24 ghonta, shob purpose) → `smsCap` (shop er timezone er din, `checkout.sms_daily_cap`) → `insertOTP` (5 minute) → `Seal` (key `otpSMSKey` = HKDF(`OTP_SECRET`), AAD otp id) → `jobs.Enqueue` `sms.send` (`max_attempts` 2).
   - Response `{"sent": true}`; limit/lock/cap e `429 rate_limited` + `Retry-After`.
8. SMS: worker er `notify.SendSMSJob` payload e `otp_id` pele `SMSCode` (`otpSMS`: phone `otp_codes` theke shop shoho, customer er `users.locale` ar shop er `translations` e `sms.otp` override; code decrypt) dake, lekha `notify.OTPText` ([jobs](jobs.md)); done hole worker payload `{}` kore ([jobs](jobs.md)).
9. OTP verify (`POST /api/auth/otp/verify {phone, code}`, Turnstile na): `handler.go` → `VerifyOTP`, tarpor `service.go` → `verifyOTP`:
   - `phoneLocked` (24 ghontay 10 bhul hole `429`, `lockAlert`) → `latestOTP` (shesh, use na hoya, expire na hoya login OTP; na pele `400 otp_invalid`) → `countAttempt` (autocommit e conditional `attempts + 1`; kichu na ele `attemptsUsed` dekhe `otp_too_many_attempts` ba `otp_invalid`) → `CheckOTP`; bhul hole abar `phoneLocked` (10 tomo bhul e `429` + alert), na hole `otp_invalid`.
   - Thik hole `db.InTx`: `consumeOTP` (conditional; parallel e ekta-i) → `upsertUser` (notun user er `locale` = `httpx.Locale`, purono user bodlay na) → `linkGuestOrders` (`user_id IS NULL` shudhu) → `insertSession` (`session.go` → `newSessionToken`: `rand.Text()`, DB te `sha256`; 30 din, `ip` = `httpx.IP`, `user_agent` 512 byte porjonto).
   - Response `{token, user}` (`User`: `phone`, `name`, `email`, `locale`; id na). Cart merge M5 e.
10. Customer route: `httpx.RequireCustomer` + `LookupSession` (`session.go`): `sessionByHash` (ei shop, `customer`, `active`, revoke na, user delete na); expire `401 session_expired`, baki `401 unauthenticated`; `last_used_at` 1 ghontar beshi purono hole `touchSession`.
11. `Logout` (`revokeSession`, `204`), `Me` (`getUser`), `UpdateMe` (`updateUser`: field na dile bodlay na, `name`/`email` `""` hole NULL; naam max 100 okkhor, email `net/mail` er ekta thikana max 254, `locale` `bn`/`en`).
12. Recovery: `recovery.go` → `NewRecoveryCodes` 10 ta `XXXXX-XXXXX`; `HashRecoveryCode(RECOVERY_SECRET, staff_id, code)` `-`/space bad, boro haat kore `HMAC-SHA256(secret, staff_id:CODE)`. DB te hash diye khoja (PK `staff_id, code_hash`), tai alada tulona function nai.
13. Staff login (`POST /api/admin/auth/login {shop, email, password}`, shop context chhara): `admin_handler.go` → `Login`: age `httpx.FailLimit.Take` (client IP te 15 minute e 10 bhul; bhul chhara shob e `Refund`), tarpor `staff.go` → `staffLogin`:
   - `staffForLogin` (slug er shop, status jai hok (closed o, Q7), `lower(email)`). Ochena shop/email ba inactive: `checkPasswordSem(dummyHash)` (600000 iteration, timing ekoi), `401 invalid_credentials`; audit `staff.login_failed` (ochena email e `staff_id` NULL, shudhu `email_domain`, email na: keu email er ghore password likhte pare; ochena shop e audit nai, shop nai).
   - PBKDF2 er age `reserveFailed` (autocommit, ek query: lock na thakle `failed_logins + 1`, 5 ba beshi hole `locked_until = greatest(..., now + 1 minute)`; `RETURNING` na ele lock). Lock: `checkPasswordSem(dummyHash)`, audit `reason: locked`, `401 invalid_credentials` (`Retry-After` na).
   - `checkPasswordSem` (`password.go`: `pbkdfSem` chan, ek shathe 4; opekkhay request bondho hole ctx error). Bhul: `passwordFailed` (gona reservation e hoye geche): 10 ar 50 e `securityAlert` (`alert.security` `{event: staff_login_failed, staff_id, count}`, `dedupe_key` `alert.staff_login_failed:<shop>:<staff>:<count>:<din>`), audit; `401 invalid_credentials`.
   - Thik: `resetFailedLogins` (`failed_logins = 0`, password er lock khole; `failed_mfa` 10 ba beshi hole lock thake), `revokeMFAPending` (purono temp_token), `newSessionToken`, `insertStaffSession`: TOTP thakle `mfa_pending` (5 minute) → `{totp_required: true, temp_token}` (body te, cookie na); na hole `setup_pending` (15 minute) → `httpx.AdminCookie` (Max-Age 900) + `{setup_required: true}`. Audit `staff.login` (`stage`).
14. TOTP ar recovery (`POST /api/admin/auth/totp` ba `/recovery {temp_token, code}`): `admin_handler.go` → `TOTP`/`Recovery` → `staff.go` → `verifyMFA`:
   - `mfaSession` (token hash e `mfa_pending`, revoke na, staff active; shop session er row theke); na pele `401 unauthenticated`, expire `401 session_expired`.
   - `countMFAAttempt` (autocommit, conditional `mfa_attempts < 5`); na hole `400 mfa_too_many_attempts`.
   - `reserveFailed` (`failed_mfa + 1`, 10 e 30 minute lock; lock thakle `429` + `Retry-After`): check er AGE, tai parallel temp_token e-o lock er age 10 tar beshi guess na.
   - TOTP: `OpenTOTPSecret` (row er `totp_key_version`) → `CheckTOTP`; recovery: `HashRecoveryCode`.
   - `db.InTx`: `useTOTPStep` (`totp_last_step` conditional, replay atkay) ba `useRecoveryCode` (`used_at` conditional) → `activateSession` (oi row-i `active` 12 ghonta, `must_change_password` hole `setup_pending` 15 minute; `token_hash` notun) → `mfaSucceeded` (`failed_mfa = 0`, `locked_until` NULL, `last_login_at`) → audit `staff.login_mfa`.
   - Bhul (ba replay/use kora code): `mfaFailed` (gona reservation e hoye geche; 10 ba beshi hole `429`), 10 e alert (`staff_mfa_lock`, `dedupe_key` `alert.staff_mfa_lock:<shop>:<staff>:<din>`), audit `staff.login_mfa_failed`; na hole `400 mfa_invalid`.
   - Thik hole `httpx.AdminCookie` (43200 ba 900) + `{setup_required}`.
15. Admin session (`cmd/api/router.go`): `httpx.AdminSession` + `session.go` → `LookupStaffSession` → `staffSessionByHash` (token hash e `active`/`setup_pending`, revoke na, staff active; shop session theke): `httpx.Staff` (`SessionID`, `Setup`); expire `401 session_expired`, baki `401 unauthenticated`; `last_used_at` ghontay ekbar. `GET /api/admin/me` (`Me` → `staffProfile`: `name`, `email`, `role`, `vendor_id`) ar baki admin route `RequireStaff` er pore (`setup_pending` e `403 setup_required`).
16. Setup (`admin_handler.go`, `staff.go`):
   - `Password {old, new}` (active ba setup_pending): new 12–128 okkhor (`utf8`), old er sathe ekoi na (`400 validation_failed`, `fields.new`) → `changePassword`: `checkPasswordSem` (bhul `fields.old: wrong`) → `hashPasswordSem` → `db.InTx`: `setPassword` (`must_change_password = false`, `password_changed_at`) → `revokeOtherSessions` (ei session chhara shob) → audit `staff.password_changed`; `204`.
   - `TOTPSetup`, `TOTPEnable` shudhu `setup_pending` (`setupOnly`, active e `403 forbidden`); `must_change_password` thakle `403 password_change_required`. `totpSetup`: `staffTOTP` (`FOR UPDATE`) → `NewTOTPSecret` → `SealTOTPSecret` (version `totpKeyVersion` 1) → `setTOTPSecret` (purono replace) → audit `staff.totp_setup`; `{otpauth_url, secret}` (`otpauth://totp/<slug>:<email>?secret=..&issuer=<slug>`).
   - `totpEnable {code}`: `staffTOTP` → `OpenTOTPSecret` → `CheckTOTP` → `useTOTPStep` (replay na) → `replaceRecoveryCodes` (purono muche, notun 10 ta hash) → `activateSession` (`setup_pending` row-i `active` 12 ghonta, token notun) → audit `staff.totp_enabled`; `AdminCookie` 43200 + `{recovery_codes}`. Bhul code `400 mfa_invalid` (`failed_mfa` na).
   - `Logout` (active ba setup_pending): `revokeSession`, `AdminCookie(w, "", 0)` (`Max-Age=0`), `204`.
17. TOTP reset CLI (`shop-api staff reset-2fa --shop <slug> --email <email>`, `cmd/api/main.go` → `runStaffReset2FA`; env shudhu `DATABASE_URL`, `config.Shop`): `staff.go` → `ResetStaff2FA` → `db.InTx`: `staffForReset` (shop slug e, staff email e boro/choto haat mane na, `FOR UPDATE`) → `clearStaff2FA` (TOTP secret/nonce/key_version/`totp_last_step` NULL, `failed_mfa` 0, recovery code muche, shob session revoke) → audit `staff.reset_2fa` (`staff_id` NULL, `entity_id` staff, `after {"actor":"cli"}`). Porer login e TOTP nai, tai `setup_pending` (step 16). Kono secret print na, shudhu log.

## Code er map

| File → function | Ki kore |
|---|---|
| `backend/internal/auth/doc.go` | Package comment |
| `backend/internal/auth/password.go` → `HashPassword`, `CheckPassword` | Staff password hash ar tulona |
| `backend/internal/auth/otp.go` → `NewOTPCode`, `HashOTP`, `CheckOTP` | 6 digit OTP, HMAC ar constant-time tulona |
| `backend/internal/auth/totp.go` → `TOTPCode`, `CheckTOTP`, `NewTOTPSecret`, `SealTOTPSecret`, `OpenTOTPSecret` | RFC 6238 code, ±1 step, secret (base32) ar AES-GCM (AAD shoho) |
| `backend/internal/auth/recovery.go` → `NewRecoveryCodes`, `HashRecoveryCode` | Recovery code ar tar hash |
| `backend/internal/auth/seal.go` → `Seal`, `Open` | AES-GCM + AAD; shop credential ([credentials](credentials.md)), TOTP secret ar OTP SMS payload |
| `backend/internal/auth/handler.go` → `NewHandlers`, `SendOTP`, `VerifyOTP`, `Logout`, `Me`, `UpdateMe`, `writeUser`, `internal`, `noStore`, `writeLimit`, `userAgent` | Customer route er HTTP, error code, `Cache-Control: no-store`, `429` + `Retry-After`, user_agent (512 byte) |
| `backend/internal/auth/service.go` → `sendOTP`, `lockAlert`, `otpSMSKey`, `SMSCode`, `verifyOTP`, `phoneLocked`, `User`, `UserPatch` | OTP send/verify er niyom, alert, SMS payload er key ar decrypt |
| `backend/internal/auth/turnstile.go` → `Turnstile.Verify` | Cloudflare siteverify |
| `backend/internal/auth/store.go` → `lockPhone`, `isBlocked`, `phoneLock`, `sendLimit`, `smsCap`, `insertOTP`, `otpSMS` | OTP send er SQL |
| `backend/internal/auth/store.go` → `latestOTP`, `countAttempt`, `attemptsUsed`, `consumeOTP`, `upsertUser`, `linkGuestOrders`, `insertSession`, `sessionByHash`, `touchSession`, `revokeSession`, `getUser`, `updateUser` | Verify, session ar `/api/me` er SQL |
| `backend/internal/auth/session.go` → `newSessionToken`, `tokenHash`, `LookupSession`, `LookupStaffSession` | Session token; `httpx.RequireCustomer` ar `httpx.AdminSession` er lookup |
| `backend/internal/auth/admin_handler.go` → `NewAdminHandlers`, `Login`, `TOTP`, `Recovery` | Staff login er HTTP, IP limit, cookie, error code |
| `backend/internal/auth/admin_handler.go` → `Password`, `TOTPSetup`, `TOTPEnable`, `setupOnly`, `setupErr`, `Logout`, `Me` | Setup, logout, `/api/admin/me` er HTTP |
| `backend/internal/auth/staff.go` → `staffLogin`, `passwordFailed`, `verifyMFA`, `mfaFailed`, `securityAlert` | Staff login, TOTP/recovery er niyom (lock, alert, audit) |
| `backend/internal/auth/staff.go` → `changePassword`, `totpSetup`, `totpEnable` | Setup er niyom (password, TOTP secret, recovery code, active) |
| `backend/internal/auth/password.go` → `checkPasswordSem`, `hashPasswordSem`, `pbkdfSlot`, `pbkdfSem`, `dummyHash` | Ek shathe 4 PBKDF2, ochena email er dummy tulona |
| `backend/internal/auth/staff_store.go` → `staffForLogin`, `reserveFailed`, `resetFailedLogins`, `revokeMFAPending`, `shopDay`, `insertStaffSession`, `mfaSession`, `countMFAAttempt`, `useTOTPStep`, `useRecoveryCode`, `activateSession`, `mfaSucceeded`, `insertAudit` | Staff login er SQL |
| `backend/internal/auth/staff_store.go` → `staffSessionByHash`, `staffPasswordHash`, `setPassword`, `revokeOtherSessions`, `staffTOTP`, `setTOTPSecret`, `replaceRecoveryCodes`, `staffProfile` | Admin session ar setup er SQL |
| `backend/internal/auth/staff.go` → `ResetStaff2FA`, `ErrShopNotFound`, `ErrStaffNotFound`; `staff_store.go` → `staffForReset`, `clearStaff2FA` | `staff reset-2fa` er niyom ar SQL |
| `backend/cmd/api/main.go` → `runStaffReset2FA` | CLI er flag, config, error |

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
| Turnstile test secret (`1x`/`2x`/`3x...AA`) e `hostname` check na (Cloudflare `example.com` dey), `success` lage; asol secret e local eo check (10.11 v1.22, 16.1) | `Turnstile.Verify`, `config.TurnstileTestSecret` | `TestTurnstile` |
| OTP SMS payload `{otp_id, nonce, ct}` (AES-GCM, key HKDF(`OTP_SECRET`), AAD otp id), plain code na; `max_attempts` 2; done hole `{}` (8.2 step 4, 8.17, Q1) | `sendOTP`, `SMSCode`, `jobs.finish` | `TestOTPJobPayload` |
| OTP SMS customer er `locale` e (user na thakle bn), shop er `sms.otp` override shudhu nijer shop er (8.16, Rule 3) | `otpSMS` | `TestOTPJobPayload` |
| Client IP te ghontay 10 OTP send; website er `X-Client-IP` shudhu secret shoho (7.8, 3.2 Niyom 5, 14.2 test 18) | `cmd/api/router.go` → `newHandler` | `TestOTPSendIPLimit` |
| OTP verify: ek OTP e 5 attempt (autocommit, error path e thake, parallel e 5 er beshi na), tarpor `otp_too_many_attempts`; 24 ghontay 10 bhul e `429` + alert ekbar, lock e thik code o na (8.2 verify step 2-3, 7.8, 14.2 test 10) | `verifyOTP`, `countAttempt`, `phoneLocked` | `TestOTPAttemptsIntegration` |
| Shudhu shesh, use na hoya, expire na hoya login OTP; expire e attempt gona na; ek code ekbar (8.2 verify step 1, 3) | `latestOTP`, `consumeOTP` | `TestOTPExpiredIntegration` |
| Ekoi code e parallel verify → ekta session, ekta user (8.2 step 3 v1.7) | `consumeOTP` | `TestOTPParallelVerifyIntegration` |
| Guest order jog shudhu thik verify er por, `user_id IS NULL`, ei shop (8.2 step 5) | `linkGuestOrders` | `TestGuestOrdersLinkIntegration` |
| Session: `rand.Text`, DB te `sha256`, 30 din, ip/user_agent; notun user er locale `Accept-Language`; logout revoke; expire `session_expired`; staff token customer route e `401`; `last_used_at` ghontay ekbar; `/api/me` validate (8.2 step 4, 7, 12.2, Rule 16, Q10 iii) | `verifyOTP`, `LookupSession`, `Logout`, `UpdateMe` | `TestCustomerSessionIntegration` |
| Onno shop er token `401` (logout o na); OTP ar user shop onujayi (3.2, Rule 3) | `sessionByHash`, `latestOTP`, `upsertUser` | `TestSessionShopIsolationIntegration` |
| Staff login: ochena shop/email ba inactive e dummy hash, shob `401 invalid_credentials`; `failed_logins`, 5 er por protiti bhul e 1 minute lock (lock e o `401`, `Retry-After` na), thik hole 0; 10 ar 50 e alert; TOTP thakle `mfa_pending` temp_token (body, 5 minute), na hole `setup_pending` + `__Host-asid` 900; shob login audit; closed/suspended shop e chole; onno shop er slug e na (8.3 step 1-4, 7.7, Q7, Q9) | `staffLogin`, `reserveFailed`, `resetFailedLogins` | `TestStaffLoginIntegration` |
| Ek shathe 4 tar beshi PBKDF2 na (8.3 step 1) | `checkPasswordSem` | `TestPBKDFSemaphore` |
| Client IP te 15 minute e 10 bhul login, shudhu bhul gone (7.8) | `Login`, `httpx.FailLimit` | `TestLoginIPLimit` |
| Ek temp_token e 5 bar (parallel e o); expire `session_expired`; thik hole oi row active 12 ghonta, token notun, cookie 43200; `must_change_password` e TOTP er por `setup_pending` (8.3 step 2-4) | `verifyMFA`, `countMFAAttempt`, `activateSession` | `TestMFAAttemptsIntegration` |
| TOTP ekoi code ba purono step duibar na (parallel o) (8.3 step 2 v1.5, 14.2 test 10) | `useTOTPStep` | `TestTOTPReplayIntegration` |
| `failed_mfa` temp_token jure 10 e 30 minute lock + alert (`dedupe_key` din shoho), notun login e purono temp_token revoke, lock e login o na, thik hole 0 (8.3 step 2 v1.24, 8.17, 14.2 test 10) | `mfaFailed`, `reserveFailed`, `revokeMFAPending` | `TestFailedMFAAcrossTokensIntegration` |
| Lock check er age atomic reservation: parallel bhul password e 5 tar beshi asol PBKDF2 na, lock e thik password o `401`; parallel temp_token e 10 tar beshi guess na, lock e thik code `429`; lambha lock choto na; login alert `dedupe_key`; `/auth/password` e bhul old gona + audit; ochena email er audit e shudhu domain; auth response `no-store` (8.3 step 1, 2, 4 v1.24, 8.17, 12.2) | `reserveFailed`, `passwordFailed`, `changePassword`, `noStore` | `TestStaffLockRaceIntegration` |
| `/auth/totp`, `/recovery` e client IP te minute e 120 (7.8 v1.24) | `cmd/api/router.go` → `newHandler` | `TestAdminMFAIPLimit` |
| Recovery code ekbar (parallel o), `-`/space/case bad, onno staff er na (8.3 step 3) | `useRecoveryCode` | `TestRecoveryCodeIntegration` |
| Setup: `setup_pending` e admin API `403 setup_required`; age password (old thik, 12–128 okkhor, purono na; onno shob session revoke, audit), `must_change` thakle TOTP `403 password_change_required`; `totp/setup` replace; enable e thik code e 10 recovery code (purono muche), active 12 ghonta, token notun; active e setup route `403`; logout revoke + `Max-Age=0`; expire `session_expired`, inactive `401` (8.3 step 4, 6, 7.7, Q10) | `changePassword`, `totpSetup`, `totpEnable`, `LookupStaffSession` | `TestStaffSetupIntegration` |
| `staff reset-2fa`: ek transaction e TOTP ar recovery code muche, shob session revoke, audit `staff.reset_2fa` (actor CLI); porer login `setup_pending`; onno shop er staff e hat na; ochena shop/email e spashto error (8.3 item 6, 7.7, Q8) | `ResetStaff2FA`, `runStaffReset2FA` | `TestStaffReset2FAIntegration`, `TestRun` |
| Session er dhoron router e: customer token admin e, `mfa_pending` admin e, staff token customer route e `401`; `setup_pending` e shudhu setup route (14.2 test 19) | `httpx.AdminSession`, `LookupStaffSession` | `TestSessionRouteKind` |
| Recovery: 10 ta, `crypto/rand` 10 okkhor base32, `XXXXX-XXXXX`; hash `HMAC-SHA256(RECOVERY_SECRET, staff_id:code)`, `-`/space/case bad (8.3 step 5, 6) | `NewRecoveryCodes`, `HashRecoveryCode` | `TestRecoveryCode` |

## Data

`staff.password_hash`; `otp_codes.code_hash` (`BYTEA`, 32 byte) — migration `001_init.sql`. `staff.totp_secret_enc`, `totp_nonce`, `totp_key_version` (`staff_totp_check`: tinta ekshathe NULL ba na), `totp_last_step`; `staff_recovery_codes.code_hash`. OTP verify `otp_codes.attempts`, `consumed_at` bodlay, `users` e row (upsert), `orders.user_id` (guest), `sessions` e customer row (`token_hash`, `expires_at`, `last_used_at`, `revoked_at`). OTP send `otp_codes` e row (`expires_at` 5 minute) ar `jobs` e `sms.send` (ba lock e `alert.security`) lekhe, `blocked_phones` pore, `shop_settings.published` er `checkout.sms_daily_cap` ar `shops.timezone` pore; index `otp_phone_idx`. Staff login `staff.failed_logins`, `failed_mfa`, `locked_until`, `totp_last_step`, `last_login_at`, `staff_recovery_codes.used_at` bodlay; `sessions` e staff row (`stage` `mfa_pending` → `active`/`setup_pending`, `mfa_attempts`, `token_hash` active hole notun); `audit_logs` (`staff.login`, `staff.login_failed`, `staff.login_mfa`, `staff.login_mfa_failed`, `after` e `reason`/`stage`/`method`); `jobs` e `alert.security`. Setup `staff.password_hash`, `must_change_password`, `password_changed_at`, `totp_secret_enc`/`totp_nonce`/`totp_key_version`, `totp_last_step`, `staff_recovery_codes` (muche notun 10), `sessions.revoked_at` (onno session), `stage` `setup_pending` → `active`; audit `staff.password_changed`, `staff.totp_setup`, `staff.totp_enabled`. `reset-2fa` TOTP column ar `failed_mfa` bodlay, `staff_recovery_codes` muche, `sessions.revoked_at`; audit `staff.reset_2fa`.

## Bhul hole ki hoy

- `CheckPassword`: bhul format, iteration 1 er kom ba 10M er beshi, bhul base64 e `false`.
- `CheckOTP`: `nil` ba choto hash, bhul code/phone/purpose/secret e `false`.
- `CheckTOTP`: 6 okkhor chhara, bhul digit, window er baire (±2 step) ba onno secret e `false`.
- `OpenTOTPSecret`: key, shop, staff ba version na mille error (GCM auth fail); `Seal`/`Open` 32/24/16 byte chhara key e error.
- `HashRecoveryCode` kokhono fail kore na; bhul code er hash DB te mile na.
- OTP send: bhul JSON ba `purpose` `400 validation_failed`; bhul phone `400 invalid_phone`; Turnstile fail `400 turnstile_failed`; Cloudflare e jete na parle ba DB error `500 internal` (log e phone mask); blocked `403 phone_blocked`; limit, lock ba cap `429 rate_limited` + `Retry-After`.
- OTP verify: bhul JSON `400 validation_failed`, bhul phone `400 invalid_phone`; OTP nai, expire, use hoye geche ba bhul code `400 otp_invalid`; 5 attempt shesh `400 otp_too_many_attempts`; lock `429 rate_limited` + `Retry-After`; DB error `500 internal` (log e phone mask).
- Customer route: token nai/bhul/onno shop/revoke/staff `401 unauthenticated`, expire `401 session_expired`; user delete hole `/api/me` `401`. `PATCH /api/me` bhul field e `400 validation_failed` (`fields` e kon ta).
- Staff login: bhul JSON `400 validation_failed`; ochena shop/email, inactive, bhul password `401 invalid_credentials`; account lock o `401 invalid_credentials` (enumeration na, 16.1 v1.24); IP limit `429 rate_limited` + `Retry-After`; DB error `500`.
- TOTP/recovery: temp_token nai/bhul/customer/active hoye geche `401 unauthenticated`, expire `401 session_expired`; 5 attempt shesh `400 mfa_too_many_attempts`; bhul, replay ba use kora code `400 mfa_invalid`; `failed_mfa` lock ba IP limit (minute e 120) `429`; `OpenTOTPSecret` fail (bhul `TOTP_KEY`) `500`.
- `staff reset-2fa`: flag kom/beshi hole usage; `DATABASE_URL` nai hole config error; ochena slug `ErrShopNotFound`, oi shop e email nai `ErrStaffNotFound` (kichu bodlay na, exit 1).
- `SMSCode`: onno shop, onno otp id ba onno `OTP_SECRET` e error (job retry, shesh e failed ar alert).
- `NewOTPCode`: `crypto/rand` fail korle panic (Go te rand fail kore na; korle chalano nirapod na).

## Bodlanor age mone rakho

- Hash er input (`phone:purpose:code`) ba secret bodlale DB te thaka chalu OTP (5 minute) ar mile na; `TestOTPHash` er openssl vector o bodlate hobe (8.2 age).
- Phone sob shomoy E.164 e dao; normalize chhara hash alada hoy.
- `OTP_SECRET` env (13.5, `config.Config.OTPSecret`); rotate korle shudhu chalu OTP gulo batil.
- TOTP er AAD format ba `TOTP_KEY` bodlale DB er shob secret decrypt hoy na: notun `totp_key_version` diye re-encrypt (8.3 step 5). `Seal`/`Open` shop credential o use kore (`TestSealOpen`).
- Turnstile er hostname check bad shudhu test secret e (`config.TurnstileTestSecret`); test secret local chhara config e atke (13.5), tai notun test secret jog korle duita jaygay.
- OTP send er limit er number `service.go` er `otpSendLimits`, `otpLockWrong`; bodlate 7.8 age (`design-change`).
- `otpSMSKey` er info ba AAD bodlale queue te thaka OTP job decrypt hoy na (5 minute e mullohin, tai shudhu deploy er muhurter OTP).
- Session token er hash (`tokenHash`) ba TTL bodlale website er `__Host-sid` Max-Age (10.3) o; hash bodlale chalu shob session batil.
- Staff login er lock er number `staff.go` er const (`loginLockAfter`, `mfaLockAfter` ...); bodlate 8.3/7.8 age. `dummyHash` er iteration `HashPassword` er shoman rakho, na hole timing e email chena jay.
- `__Host-asid` er flag shudhu `httpx.AdminCookie` e; Max-Age `adminTTL`/`setupTTL` er sathe mile (11.3).
- Verify er attempt `countAttempt` transaction er baire-i thakbe (8.2 step 2); transaction e nile `ROLLBACK` e attempt muche jay.
- Recovery hash er input (normalize, `staff_id:code`) ba `RECOVERY_SECRET` bodlale shob staff er code batil; `TestRecoveryCode` er vector o bodlao.

## Proshno-uttor

- **Test secret e hostname check na keno?** Cloudflare er test secret shob shomoy `example.com` dey, tai local login hoto na; test secret local chhara config e atke (v1.20), tai production e check bad hoy na (10.11).
- **Verify e Turnstile na keno?** OTP nijei 5 attempt ar 24 ghontar lock e bandha; Turnstile send e (14.2 test 23).
- **Lock er 10 tomo bhul e `429`, `otp_invalid` na keno?** Tokhon theke lock chalu; client `Retry-After` dekhe opekkha kore, lock ache bole na (7.8).
- **Response e user id nai keno?** Client er lage na; serial id baire dei na (Rule 11 er moto).

- **TOTP er por token bodlay keno?** Temp_token response body te JS dekhe; oi token-i HttpOnly cookie hole XSS e pawa temp_token pore admin session hoto. Row ekoi (8.3 step 3), shudhu `token_hash` notun.
- **TOTP thakle `must_change_password` holeo age TOTP keno?** 8.3 step 4 "password (ar TOTP thakle TOTP)": na hole shudhu password e `setup_pending` pele `/auth/totp/setup` e TOTP bodle dewa jeto.
- **Setup e TOTP enable er age-i secret staff e keno?** 8.3 step 6 "enable er age purono secret replace"; alada pending column nai. Tai setup er por enable na korleo porer login TOTP chay (scan na korle `reset-2fa`).
- **Active session e `totp/setup` keno na?** Churi kora cookie diye TOTP bodle dewa jeto; TOTP reset `reset-2fa` (CLI, M7 e `/staff`).
- **`reset-2fa` e `failed_mfa` 0 keno, `locked_until` na keno?** `failed_mfa` purono TOTP er bhul; lock nijei khule (password er lock o oi column e).
- **`reset-2fa` e `must_change_password` na keno?** Password harano na, shudhu TOTP; dorkar hole M7 er `/staff` e.
- **Enable e bhul code e `failed_mfa` na keno?** `setup_pending` password e pawa, TOTP chara-i; lock er kono labh nai.
- **Staff alert er `dedupe_key` e din keno?** 8.17: key unique chirokal, din chhara porer bar (thik login e gona 0, abar 10) alert atkato; din shoho ek dine ekbar.
- **Lock check `reserveFailed` e, aage SELECT e na keno?** SELECT er snapshot e parallel request shob "lock nai" dekhto: password burst e shob PBKDF2 chalto, parallel temp_token e 10 er beshi TOTP guess hoto (M4 step 15 review). Conditional `UPDATE ... RETURNING` atomic, tai lock er age thik 5 (ba 10) ta check.
- **Account lock e `401`, `429` na keno?** `429` ar thik/bhul password e alada response dekhe email ache kina ar password thik kina chena jeto; lock e o dummy PBKDF2, tai shomoy o ekoi (user decision, 16.1 v1.24). TOTP er lock e `429` thake: temp_token er malik password jane.

- **Code `string` keno, `int` na?** Samne zero (`012345`) thakte hobe; SMS, hash ar input shob ekoi lekha.
- **TOTP library keno na?** 2.2: stdlib + pgx chhara dependency na; RFC 6238 20 line, appendix B vector diye test.
- **Limit e alada alada error na keno?** Shob `429 rate_limited`: lock ache naki limit, phone er obostha bole na (7.8).
- **Blocked phone `403` keno, `{"sent": true}` na?** M4 er scope e `phone_blocked` (7.1); admin er block customer ke jante hobe.
- **±1 step keno?** Phone ar server er ghori 30 second porjonto agepiche hoy; beshi window e anda korar shujog bare.
- **`==` na `hmac.Equal` keno?** `==` prothom bhul byte e thame; shomoy mepe hash anda kora jay.
- **Customer logout `204` keno, body na?** Dewar moto kichu nai; website er BFF Go er status jai hok cookie `Max-Age=0` kore ([website](website.md)).
- **`PATCH /api/me` e ki check hoy?** `name` trim, max 100 okkhor (rune); `email` `net/mail` e shudhu thikana (`"Naam <a@b>"` na), max 254 (SMTP er shima); `""` dile NULL (muche); `locale` `bn`/`en`; field na dile bodlay na. Bhul gulo ek shathe `fields` e.
- **Staff soft lock 5 tomo bhul thekei 1 minute keno?** `failed_logins + 1 >= 5` hole protiti bhul e `locked_until = now + 1 minute` (8.3 step 1, 7.8): brute force minute e ekta-y name, kintu keu owner er email diye bhul dileo owner shudhu 1 minute opekkha kore.
- **`failed_mfa` 10 e 30 minute keno, alada column na keno?** 8.3 step 2: temp_token jure gona, na hole notun login e notun 5 attempt. Lock ekoi `locked_until` e, tai lock e password login o `401`; thik password `resetFailedLogins` `failed_mfa` er lock khole na.
- **Replay ba use kora recovery code o `failed_mfa` e gone keno?** Bairer lok er kache ei duita bhul code-er motoi; na gunle purono code diye attempt barano jeto.
- **Recovery code e `-`, space, boro/choto haat bad keno?** Staff kagoj theke je bhabe-i likhuk (`abcde fghij`) ekoi hash; alphabet base32, tai `0/O`, `1/I` gulie jay na.
- **Bhul old password e `401` na, `400 validation_failed` (`fields.old`) keno?** Session thik ache; `401` hole admin client logout kore dito. Ei route e `failed_logins` gone na (session lage).
- **otpauth URL e issuer shop er slug keno?** Ek staff er onek shop e account thakle authenticator app e alada dekhay.
