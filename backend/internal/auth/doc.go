// Package auth: login er primitive (8.2, 8.3): staff password hash, customer OTP code ar hash, TOTP
// (RFC 6238) ar tar secret er AES-GCM, recovery code; POST /api/auth/otp/send (limit, lock, Turnstile, SMS job).
// Verify, session ar staff login M4 er porer step e.
//
// Feature doc: docs/features/auth.md
package auth
