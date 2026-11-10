# packages/shared (pure Dart, M2)

Website, admin ar app er shared code: model, API client, l10n (translator), money/tarikh format, validation, theme token (4.3, 9.3, 9.8). Ekhono code nai; M2 e shuru (`milestone` skill).

M2 er shuru:
1. Pure Dart package (`dart create -t package`, ei README notun README diye bodlao). Dependency shudhu `http`, `intl` (Rule 10, 2.3).
2. `analysis_options.yaml` e strict lint ar `public_member_api_docs` (shob app er shared code).
3. Age test: money format (`৳১,২৫,০০০`, `৳১৮০.৫০`, `৳1,25,000`; `NumberFormat.currency` na, 9.8), translator fallback, contrast, phone.
4. Feature doc `docs/features/` e, index ar code map e row; `.github/workflows/ci.yml` e Dart job.

Website, admin, app eta path dependency diye ney: `shared: {path: ../packages/shared}` (4.6). `.claude/rules/shared-dart.md` er niyom.
