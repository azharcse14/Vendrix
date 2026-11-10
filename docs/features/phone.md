# Phone number

Design: 8.1 (Rule 6) · Milestone: login (M4) ar checkout (M6) e use · Code: `backend/internal/phone`, `packages/shared/lib/src/validation` (M2)

## Ki kore

Je format e phone ashuk (dash, space, `880`, `00880`, Bangla shonkhya, phone theke copy kora lukano chinho), save er age `+8801XXXXXXXXX` (E.164) e ane; bhul hole `ErrInvalid`.

## Kivabe kaj kore

1. `Normalize` protiti okkhor dekhe: digit (Bangla shonkhya o) rakhe; `+` shudhu shuru te; lukano format chinho (Unicode Cf) bad; space, dash, `.`, `(`, `)` separator; onno kichu thakle bhul.
2. `+` thakle tarpor `880` thakte hobe; `00880` theke `00` bad; `880` bad; 10 digit `1...` hole samne 0.
3. Shesh e 11 digit `01[3-9]...` hole `+88` + number.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/phone/phone.go` → `Normalize` | Normalize |
| `backend/internal/phone/phone_test.go` | Thik ar bhul input |
| `packages/shared/lib/src/validation/phone.dart` → `normalizePhone` | Ekoi niyom Dart e (form e sathe sathe check; bhul hole `null`), v1.12 |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| E.164 e save (Rule 6, 8.1) | `Normalize` | `TestNormalize` |
| Bhul okkhor chupchap fele na (8.1) | `Normalize` | `TestNormalize` |
| `+` er por `880`, ekbar-i (8.1) | `Normalize` | `TestNormalize` |
| Dart er niyom Go er sathe ekoi, ekoi case (8.1, v1.12) | `normalizePhone` | `test/validation/phone_test.dart` → `group('normalizePhone (8.1, Go er sathe ekoi)')` |

## Data

`phone_bd` domain (`001_init.sql`); database-o bhul format atkay (`001_init_test.sql`).

## Bhul hole ki hoy

`ErrInvalid` → handler `400 invalid_phone` (7.1).

## Bodlanor age mone rakho

- Notun operator prefix hole `Normalize`, Dart er `normalizePhone` ar `phone_bd` domain tinta-i (domain er jonno notun migration).
- Go er `TestNormalize` er case bodlale `packages/shared/test/validation/phone_test.dart` e hubohu copy (v1.12); server sob shomoy Go diye abar check kore, Dart shudhu form er jonno.
- Log e phone mask (`+88017****5678`, 12.2): mask er function M4 e ashbe (ekhono nai).

## Proshno-uttor

- **Bangla shonkhya keno?** Customer Bangla keyboard e "০১৭..." likhte pare.
