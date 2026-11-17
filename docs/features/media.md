# Chobi upload ar storage

Design: 2.2 (upload, R2 SigV4), 7.7 (Product), 11.7 item 9, 12.2 item 8, 13.5, 14.2 test 29 · Milestone: M7a · Code: `backend/internal/storage`, `backend/internal/catalog` (`image.go`)

## Ki kore

Admin (owner, admin, vendor_owner nijer product e) product e JPEG/PNG chobi tole (max 10), kram bodlay ar muche. Server chobi decode kore abar encode kore (EXIF/GPS jay), object store e rakhe: staging/production e Cloudflare R2, local e `backend/.data/uploads` (Caddy `img.localhost` serve kore, M7a step 14). Website `IMAGE_BASE_URL` + `storage_key` diye dekhay.

## Kivabe kaj kore

1. `image.go` → `UploadImage` (`POST /api/admin/products/{id}/images`, multipart `file`, `alt_bn`, `alt_en`): `http.ResponseController` diye read ar write deadline 2 minute (server er 15s/30s er bodle, 2.2).
2. `imageRoom` (`lockProduct` autocommit e): product shop/vendor scope e na thakle `404`, 10 ta chobi thakle `400 images=max`, body porar age.
3. `h.sem` (cap 2) e jayga (request cancel e chhere dey; `defer` e ferot, panic holeo), tarpor `http.MaxBytesReader` 10 MB ar `readImageForm`: `r.MultipartReader()` stream (temp file na); shudhu ekta `file`, `alt_bn`, `alt_en` (trim, ≤ 200 rune).
4. `reencodeImage` (`reencode`): `image.DecodeConfig` e format (shudhu `jpeg`/`png`) ar maap (≤ 4000×4000) pixel decode er age; `image.Decode` → JPEG quality 85 ba PNG encode.
5. Key `shops/{shop_id}/products/{product_id}/{rand.Text()}.{jpg|png}` → `store.Put`.
6. `audited` tx: `imageRoom` abar (product `FOR UPDATE`, race e 11 ta na), `insertImage` (`sort_order` = max + 1), audit `image.create`. Tx fail hole `store.Delete` (request cancel holeo), log.
7. `OrderImages` (`PATCH /products/{id}/images/order {ids}`): product lock, `ids` = product er shob chobi (duplicate/baki/onno na), `sort_order` = index; audit `product.images_order`; `200 {ids}`.
8. `DeleteImage` (`DELETE /images/{id}`): row age muche (vendor scope product theke, audit `image.delete`, commit), tarpor `store.Delete`; fail hole shudhu log, `204`.
9. `storage`: `Local` (`os.Root` e, dir er baire jete pare na) ar `R2` (`PUT`/`DELETE` path-style `/{bucket}/{key}`, `signV4` region `auto`, service `s3`, `x-amz-content-sha256`, client timeout 30s; DELETE e `404` ok). Router er `newStore` `STORAGE` dekhe.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/catalog/image.go` → `UploadImage`, `readImageForm`, `bodyErr`, `reencode` | Upload, multipart, decode/re-encode |
| `image.go` → `lockProduct`, `imageRoom`, `insertImage` | Product lock, max 10, row |
| `image.go` → `OrderImages`, `orderImages`, `DeleteImage`, `deleteImage` | Kram ar muche fela |
| `backend/internal/catalog/admin.go` → `Routes`, `audited` | Route (owner, admin, vendor_owner), tx + audit |
| `backend/internal/storage/storage.go` → `Store`, `validKey`, `Local` | Interface, local dir |
| `backend/internal/storage/r2.go` → `NewR2`, `R2.Put`, `R2.Delete`, `signV4`, `uriEncode` | R2, SigV4 |
| `backend/cmd/api/router.go` → `newStore` | `STORAGE` theke store |
| `backend/internal/config/config.go` → `Load` | `STORAGE`, `UPLOAD_DIR`, `R2_*` |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| JPEG/PNG decode + re-encode, EXIF nai; SVG/WebP/PDF/bhanga `400 file=unsupported`; 4000 px er beshi `400 file=too_large`; 10 MB er beshi `413` (MaxBytesReader, multipart er age); file nai/duita/ochena part/alt lomba `400` (2.2, 12.2 item 8) | `readImageForm`, `reencode` | `TestImageUploadIntegration` |
| Max 10 (`400 images=max`), onno vendor/shop er product ba chobi `404`, DB fail e object muche, key er akar, order PATCH er id set, DELETE row age tarpor object, audit (11.7 item 9, Rule 13) | `imageRoom`, `orderImages`, `deleteImage` | `TestImageUploadIntegration` |
| Body pora, decode/re-encode, Put ekshathe max 2 (slot body porar age; panic e o ferot) (11.7 item 9) | `Handlers.sem` | `TestImageUploadSemaphore` |
| Server timeout er beshi shomoy newa upload er-o response (14.2 test 29) | `UploadImage` (`ResponseController`) | `TestImageUploadDeadline` |
| SigV4 AWS er published vector e mile; R2 request er akar (2.2) | `signV4`, `R2` | `TestSigV4`, `TestR2Store` |
| Local store key dir er baire na (11.7 item 9) | `validKey`, `Local` | `TestLocalStore` |
| `STORAGE=local` shudhu `APP_ENV=local` e; r2 e `R2_*` lagbe (13.5) | `config.Load` | `TestLoadStorage`, `TestRejectExampleSecrets` |
| Route er role (11.2) | `Routes` | `TestAdminRouteRoles` |

## Data

`product_images` (`storage_key`, `alt` JSONB `{bn, en}`, `width`, `height`, `sort_order`; `001_init.sql`), migration nai. Lekha `audit_logs` e (`image.create`, `image.delete`, `product.images_order`). Object: R2 bucket ba `UPLOAD_DIR`.

## Bhul hole ki hoy

- Product nai/onno vendor ba shop: `404 not_found` (body pora hoy na).
- Bhul file: `400 validation_failed` `fields.file` (`required`, `unsupported`, `too_large`); 10 ta: `fields.images: max`; multipart bhul: `fields.body`.
- 10 MB er beshi: `413 validation_failed`.
- R2 down: upload `500` (row nai); delete e row muche, object er error log e (orphan object, R2 versioning 13.6).
- `STORAGE=local` staging/production e, ba r2 e `R2_*` nai: `serve` start hoy na.

## Bodlanor age mone rakho

- Key er prefix `shops/{shop_id}/` (6.7 comment); bodlale website er `IMAGE_BASE_URL` ar Caddy (`img.localhost`) dekho.
- Notun format (WebP) chaile stdlib e decoder nai (2.2); design age.
- `uploadDeadline` server er `WriteTimeout` er beshi rakhte hobe; Caddy er timeout o (13.3).
- Semaphore slot body porar age theke response porjonto (Put, DB shoho): memory e ekshathe max 2 ta upload; worst case decode 4000×4000 16-bit PNG ≈ 128 MB protiti.
- `POST /uploads` (logo, certificate; M8) ei `storage.Store` use korbe.

## Proshno-uttor

- **Chobi resize hoy?** Na; Cloudflare image resizing URL diye (2.2).
- **EXIF keno muche?** Phone er chobi te GPS thake (krishok/dokan er thikana); re-encode e metadata jay.
- **Local e chobi kothay?** `backend/.data/uploads` (gitignore); `.env` e `STORAGE=local`, `UPLOAD_DIR=.data/uploads`.
