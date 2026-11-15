# Login (auth)

Design: 8.2, 8.3, 7.8, 12.2 · Milestone: M4 · Code: `backend/internal/auth`

## Ki kore

Customer phone + OTP diye login kore, staff email + password (+ TOTP) diye. Ekhon ache primitive: staff password er hash, OTP er code ar hash, TOTP (RFC 6238) ar tar secret er encryption, recovery code. OTP send/verify, session, staff login, TOTP setup ar recovery er endpoint M4 er porer step e ([M4](../milestones/M4.md)).

## Kivabe kaj kore

1. Staff password (`shop create` e owner): `password.go` → `HashPassword` (pbkdf2-sha256, 600000, 16 byte salt); tulona `CheckPassword`.
2. OTP code: `otp.go` → `NewOTPCode`: `crypto/rand` e 0..999999, `%06d` (samne zero thake).
3. OTP hash: `HashOTP(OTP_SECRET, phone, purpose, code)` = `HMAC-SHA256(secret, phone:purpose:code)`; `otp_codes.code_hash` e jabe (step 4). Phone `phone.Normalize` er por E.164.
4. Verify: `CheckOTP` notun hash banay ar `hmac.Equal` (constant time) diye tulona.
5. TOTP: `totp.go` → `TOTPCode(secret, step)` (HMAC-SHA1, dynamic truncation, 6 digit); `CheckTOTP(secret, code, now)` step `now/30` er -1, 0, +1 dekhe, mille sei step dey (replay atkate `staff.totp_last_step` e, step 6).
6. TOTP secret: `NewTOTPSecret` 20 byte random + base32 (padding chhara); DB te `SealTOTPSecret` (`seal.go` → `Seal`, AES-GCM, key `TOTP_KEY`, AAD `shop_id:staff_id:key_version`), pora `OpenTOTPSecret`.
7. Recovery: `recovery.go` → `NewRecoveryCodes` 10 ta `XXXXX-XXXXX`; `HashRecoveryCode(RECOVERY_SECRET, staff_id, code)` `-`/space bad, boro haat kore `HMAC-SHA256(secret, staff_id:CODE)`. DB te hash diye khoja (PK `staff_id, code_hash`), tai alada tulona function nai.

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
| `backend/internal/auth/seal.go` → `Seal`, `Open` | AES-GCM + AAD; shop credential ([credentials](credentials.md)) ar TOTP secret |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Password: pbkdf2-sha256, 600000, 16 byte salt, format `pbkdf2-sha256$600000$<salt>$<hash>`, constant-time tulona, bhul format e false (8.3) | `HashPassword`, `CheckPassword` | `TestPasswordHash` |
| OTP `crypto/rand`, 6 digit, samne zero shoho (8.2 step 2) | `NewOTPCode` | `TestOTPCode` |
| `code_hash = HMAC-SHA256(OTP_SECRET, phone:purpose:code)`; phone, purpose, code ba secret bodlale mile na; tulona `hmac.Equal` (8.2 step 3, verify step 3) | `HashOTP`, `CheckOTP` | `TestOTPHash` |
| TOTP RFC 6238: HMAC-SHA1, 30 second, 6 digit, ±1 step (±2 na), 6 digit chhara na; secret 20 byte, base32 (8.3 step 2, 6) | `TOTPCode`, `CheckTOTP`, `NewTOTPSecret` | `TestTOTP` (appendix B vector) |
| TOTP secret AES-GCM `TOTP_KEY`, AAD `shop_id:staff_id:key_version`; onno staff/shop/version/key e decrypt na (8.3 step 5) | `SealTOTPSecret`, `OpenTOTPSecret`, `Seal`, `Open` | `TestTOTPSecretAAD` |
| Recovery: 10 ta, `crypto/rand` 10 okkhor base32, `XXXXX-XXXXX`; hash `HMAC-SHA256(RECOVERY_SECRET, staff_id:code)`, `-`/space/case bad (8.3 step 5, 6) | `NewRecoveryCodes`, `HashRecoveryCode` | `TestRecoveryCode` |

## Data

`staff.password_hash`; `otp_codes.code_hash` (`BYTEA`, 32 byte) — migration `001_init.sql`. `staff.totp_secret_enc`, `totp_nonce`, `totp_key_version` (`staff_totp_check`: tinta ekshathe NULL ba na), `totp_last_step`; `staff_recovery_codes.code_hash`. Ekhon shudhu function; table e lekha step 4 theke (OTP), 6-7 (TOTP, recovery).

## Bhul hole ki hoy

- `CheckPassword`: bhul format, iteration 1 er kom ba 10M er beshi, bhul base64 e `false`.
- `CheckOTP`: `nil` ba choto hash, bhul code/phone/purpose/secret e `false`.
- `CheckTOTP`: 6 okkhor chhara, bhul digit, window er baire (±2 step) ba onno secret e `false`.
- `OpenTOTPSecret`: key, shop, staff ba version na mille error (GCM auth fail); `Seal`/`Open` 32/24/16 byte chhara key e error.
- `HashRecoveryCode` kokhono fail kore na; bhul code er hash DB te mile na.
- `NewOTPCode`: `crypto/rand` fail korle panic (Go te rand fail kore na; korle chalano nirapod na).

## Bodlanor age mone rakho

- Hash er input (`phone:purpose:code`) ba secret bodlale DB te thaka chalu OTP (5 minute) ar mile na; `TestOTPHash` er openssl vector o bodlate hobe (8.2 age).
- Phone sob shomoy E.164 e dao; normalize chhara hash alada hoy.
- `OTP_SECRET` env (13.5, `config.Config.OTPSecret`); rotate korle shudhu chalu OTP gulo batil.
- TOTP er AAD format ba `TOTP_KEY` bodlale DB er shob secret decrypt hoy na: notun `totp_key_version` diye re-encrypt (8.3 step 5). `Seal`/`Open` shop credential o use kore (`TestSealOpen`).
- Recovery hash er input (normalize, `staff_id:code`) ba `RECOVERY_SECRET` bodlale shob staff er code batil; `TestRecoveryCode` er vector o bodlao.

## Proshno-uttor

- **Code `string` keno, `int` na?** Samne zero (`012345`) thakte hobe; SMS, hash ar input shob ekoi lekha.
- **TOTP library keno na?** 2.2: stdlib + pgx chhara dependency na; RFC 6238 20 line, appendix B vector diye test.
- **±1 step keno?** Phone ar server er ghori 30 second porjonto agepiche hoy; beshi window e anda korar shujog bare.
- **`==` na `hmac.Equal` keno?** `==` prothom bhul byte e thame; shomoy mepe hash anda kora jay.
