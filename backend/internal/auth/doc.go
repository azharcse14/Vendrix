// Package auth: login er primitive (8.2, 8.3): staff password hash, customer OTP code ar hash, TOTP
// (RFC 6238) ar tar secret er AES-GCM, recovery code; POST /api/auth/otp/send (limit, lock, Turnstile, SMS job).
// POST /api/auth/otp/verify, customer session (LookupSession), logout, /api/me. Staff login (8.3): password,
// temp_token, TOTP ba recovery code, failed_logins/failed_mfa lock, audit; setup (password, TOTP setup/enable),
// admin session (LookupStaffSession), logout, /api/admin/me.
//
// Feature doc: docs/features/auth.md
package auth
