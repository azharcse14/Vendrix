# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **Milestone M0 (Project setup): cholche.** Plan ar progress: [M0](milestones/M0.md).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M0 step 1–3 shesh: pgx, local e migrate + `/healthz`, runner integration test. Local Postgres host port **5433** (5432 e onno project). Porer: job worker.

## Porer kaj

1. **M0 step 4, job worker ar notify** (8.17, 6.4 er claim ar sweep query): [M0](milestones/M0.md) e bistarito.
2. **M0 step 5, `credentials set` CLI** (3.8, 2.2).
3. **M0 step 6, CI**: user GitHub e push korle `ci` workflow er result; red hole thik koro.
4. M0 er acceptance er proman, `make check`, `design-reviewer`, report; tarpor **M1** (`milestone` skill).

## User er kache proshno

- **M4 er age**: OTP SMS er code `jobs.payload` e plain text thake; AES-GCM naki job chhara pathano (16.1).
- **M7 er age**: COD e packing e beshi ojon er shima; packed fulfillment undo (16.1).
- **M10 er age**: SSLCommerz validation er HTTP call DB lock er bhetore naki baire (16.1).
- **M12 er age**: production domain (doc e `tomarshop.com` placeholder), server region, SMS provider (16.1).
- **Public launch er age**: IP-bhittik rate limit ar CGNAT (16.1).

## Milestone

| # | Naam | Obostha | Plan |
|---|---|---|---|
| M0 | Project setup | cholche (10 November 2026 theke) | [M0](milestones/M0.md) |
| M1 | Shop, vendor, settings, catalog API | baki | |
| M2 | Shared Dart package | baki | |
| M3 | Website catalog | baki | |
| M4 | Login | baki | |
| M5 | Cart ar wishlist | baki | |
| M6 | Checkout (COD) | baki | |
| M7 | Admin panel v1 | baki | |
| M8 | Store settings | baki | |
| M9 | Review | baki | |
| M10 | SSLCommerz | baki | |
| M11 | Taka, support, compliance | baki | |
| M12 | Ops ar launch | baki | |
| MV | Marketplace (Phase 3) | baki | |
| MT | Multi-tenant (Phase 3) | baki | |

## Session log

- 10 November 2026: M0 step 1 (pgx v5.12.0, `driver.go`, CI go cache), step 2 (local Postgres 5433 e, migrate duibar, `/healthz` ok), step 3 (`TestMigrateIntegration` pass, shop_app grant check jog); `check-rules.sh` macOS bash 3.2 e chole na (mapfile) thik. quick-check green.
- 10 November 2026: Repo setup (design v1.8): docs, Claude Code er puro setup, CI, infra; M0 er shuru code (config, httpx, db runner, cmd/api) ar 7 ta feature doc. Go test green, DB test 83/83.
