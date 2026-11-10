# website (Jaspr, M3)

Customer website: Jaspr server mode (SSR) + Tailwind v3.4, ar browser er jonno BFF (Section 10). Niyom: `.claude/rules/website-jaspr.md`; Jaspr syntax `website/.claude/skills/` (`jaspr install-skills --agent claude-code`, Jaspr update e abar chalao) ba https://docs.jaspr.site.

## Chalano (local)
1. Lage: Dart 3.13 (Flutter er), `dart pub global activate jaspr_cli 0.23.5`.
2. API: `make db-up`, `make migrate`, `make run` (backend README).
3. `cp website/env.example website/.env`, tarpor `make web`: Tailwind `--watch` ar `jaspr serve` → http://localhost:8081.

## File
- `lib/main.server.dart`, `lib/main.client.dart`: entry; `*.options.dart` `jaspr_builder` banay (`dart run build_runner build`), commit hoy.
- `lib/app.dart`: route; `lib/pages/`, `lib/components/`, `lib/services/`, `lib/seo/` (4.4).
- `tailwind.config.js` (9.3: shudhu CSS variable), `tailwind.css` → `web/styles.css` (`bash scripts/tailwind.sh`, pin kora v3.4.19 + sha256; output git e na).
- `shared: {path: ../packages/shared}`, nijer `pubspec.lock` (4.6).
