# Login (auth)

Design: 8.2, 8.3, 7.8, 12.2 · Milestone: M4 · Code: `backend/internal/auth`

## Ki kore

Customer phone + OTP diye login kore, staff email + password (+ TOTP) diye. Ekhon ache: staff password er hash ar OTP er code ar hash er primitive. OTP send/verify, session, TOTP, recovery ar staff login M4 er porer step e ([M4](../milestones/M4.md)).

## Kivabe kaj kore

1. Staff password (`shop create` e owner): `password.go` → `HashPassword` (pbkdf2-sha256, 600000, 16 byte salt); tulona `CheckPassword`.
2. OTP code: `otp.go` → `NewOTPCode`: `crypto/rand` e 0..999999, `%06d` (samne zero thake).
3. OTP hash: `HashOTP(OTP_SECRET, phone, purpose, code)` = `HMAC-SHA256(secret, phone:purpose:code)`; `otp_codes.code_hash` e jabe (step 4). Phone `phone.Normalize` er por E.164.
4. Verify: `CheckOTP` notun hash banay ar `hmac.Equal` (constant time) diye tulona.

## Code er map

| File → function | Ki kore |
|---|---|
| `backend/internal/auth/doc.go` | Package comment |
| `backend/internal/auth/password.go` → `HashPassword`, `CheckPassword` | Staff password hash ar tulona |
| `backend/internal/auth/otp.go` → `NewOTPCode` | 6 digit OTP |
| `backend/internal/auth/otp.go` → `HashOTP`, `CheckOTP` | OTP er HMAC ar constant-time tulona |
| `backend/internal/auth/seal.go` → `Seal`, `Open` | AES-GCM + AAD; shop credential ([credentials](credentials.md)) ar TOTP secret |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Password: pbkdf2-sha256, 600000, 16 byte salt, format `pbkdf2-sha256$600000$<salt>$<hash>`, constant-time tulona, bhul format e false (8.3) | `HashPassword`, `CheckPassword` | `TestPasswordHash` |
| OTP `crypto/rand`, 6 digit, samne zero shoho (8.2 step 2) | `NewOTPCode` | `TestOTPCode` |
| `code_hash = HMAC-SHA256(OTP_SECRET, phone:purpose:code)`; phone, purpose, code ba secret bodlale mile na; tulona `hmac.Equal` (8.2 step 3, verify step 3) | `HashOTP`, `CheckOTP` | `TestOTPHash` |

## Data

`staff.password_hash`; `otp_codes.code_hash` (`BYTEA`, 32 byte) — migration `001_init.sql`. Ekhon shudhu function; table e lekha step 4 theke.

## Bhul hole ki hoy

- `CheckPassword`: bhul format, iteration 1 er kom ba 10M er beshi, bhul base64 e `false`.
- `CheckOTP`: `nil` ba choto hash, bhul code/phone/purpose/secret e `false`.
- `NewOTPCode`: `crypto/rand` fail korle panic (Go te rand fail kore na; korle chalano nirapod na).

## Bodlanor age mone rakho

- Hash er input (`phone:purpose:code`) ba secret bodlale DB te thaka chalu OTP (5 minute) ar mile na; `TestOTPHash` er openssl vector o bodlate hobe (8.2 age).
- Phone sob shomoy E.164 e dao; normalize chhara hash alada hoy.
- `OTP_SECRET` env (13.5, `config.Config.OTPSecret`); rotate korle shudhu chalu OTP gulo batil.

## Proshno-uttor

- **Code `string` keno, `int` na?** Samne zero (`012345`) thakte hobe; SMS, hash ar input shob ekoi lekha.
- **`==` na `hmac.Equal` keno?** `==` prothom bhul byte e thame; shomoy mepe hash anda kora jay.
