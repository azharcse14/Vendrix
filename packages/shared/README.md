# packages/shared (pure Dart)

Website, admin ar app er shared code: model, API client, l10n (translator), taka/tarikh format, validation, theme token (4.3, 9.3, 9.8). Dependency shudhu `http`, `intl` (Rule 10, 2.3); dev e `test`, `lints`.

Website, admin, app eta path dependency diye ney: `shared: {path: ../packages/shared}` (4.6). Library tai `pubspec.lock` commit hoy na. Niyom: `.claude/rules/shared-dart.md`; plan [M2](../../docs/milestones/M2.md).

Check: `dart pub get && dart format . && dart analyze && dart test` (ba repo root e `make quick-check`).
