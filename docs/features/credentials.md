# Shop er credential

Design: 2.2, 3.8, 12.2, 13.5 · Milestone: M0 · Code: `backend/internal/shop`, `backend/cmd/api`

## Ki kore

Shop er SMS, payment ar courier er secret (API key, password) database er `shop_credentials` e encrypt kore rakhe. Ekhon set korar upay CLI: `shop-api credentials set --shop <slug> --provider <provider>`, secret JSON stdin theke. Admin screen M8 e; API theke secret kokhono ferot jay na.

## Kivabe kaj kore

1. `run` → `runCredentials` (`cmd/api/main.go`): `set`, `--shop`, `--provider` check; tarpor `config.Load(lookup, config.Credentials)` (`DATABASE_URL`, `CREDENTIALS_KEY` base64 e 32 byte).
2. Stdin theke secret (64 KB porjonto). Stdin, karon argument shell history ar process list e dekha jay.
3. `shop.SetCredential`: secret JSON object kina, slug theke `shops.id`, `seal` (AES-GCM, random nonce, AAD = `shop_id:provider:key_version`), `shop_credentials` e upsert. Provider er list database er CHECK e.
4. Log e shudhu shop ar provider; secret kothao na.

Udahoron (local): `cd backend && set -a && . ./.env && set +a && echo '{"api_key":"..."}' | go run ./cmd/api credentials set --shop sobujbazar --provider sms`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/main.go` → `runCredentials` | CLI |
| `backend/internal/config/config.go` → `Load` (`Credentials`) | Env check |
| `backend/internal/shop/credentials.go` → `SetCredential`, `seal`, `open`, `aad` | Encrypt ar save |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| AES-GCM, AAD = `shop_id:provider:key_version`; onno row, provider, version ba key e decrypt na (2.2) | `seal`, `open`, `aad` | `TestSealOpen` |
| Slug er shop, JSON object, upsert, provider CHECK (3.8) | `SetCredential` | `TestSetCredentialIntegration` |
| `CREDENTIALS_KEY` base64 e 32 byte; error e key na (13.5, 12.2) | `config.Load` | `TestLoadCredentials` |
| CLI er argument (3.8) | `runCredentials` | `TestRun` |

## Data

`shop_credentials` (`001_init.sql`): `(shop_id, provider)` primary key, `secret_enc`, `nonce`, `key_version`, `updated_at`. `shop_app` er SELECT, INSERT, UPDATE onumoti.

## Bhul hole ki hoy

- Ochena slug: `shop "<slug>" nai`. Ochena provider: database CHECK er error (`provider "<p>": ...`).
- Secret JSON object na: `secret JSON object dao`. 64 KB er beshi: error.
- `CREDENTIALS_KEY` nai ba 32 byte na: start e error, key er value error e thake na.

## Bodlanor age mone rakho

- `key_version` ekhon shob shomoy 1 (`keyVersion`). Key rotation (13.5) korte ekadhik key (version → key) lagbe; tokhon config, `SetCredential` ar re-encrypt er kaj.
- Credential pora (`open` diye) M4 (SMS) ar M10 (SSLCommerz) e ashbe; AAD te row er `key_version` nibe (const `keyVersion` na), na hole rotation er por purono row decrypt hobe na.
- Terminal e secret type korle screen e dekha jay; file theke dao (`< secret.json`), tarpor file muche dao.

## Proshno-uttor

- **Key harale?** Purono shob credential decrypt hobe na; notun key diye shob provider er secret abar set korte hobe.
