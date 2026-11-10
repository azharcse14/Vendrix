# website (Jaspr, M3)

Customer website: Jaspr server mode (SSR) + Tailwind, ar browser er jonno BFF endpoint (Section 10). Ekhono code nai; M3 e shuru (`milestone` skill).

M3 er shuru:
1. Jaspr server mode project (`jaspr: ^0.23.5`, `pubspec.lock` commit, 2.1). Command ar option https://docs.jaspr.site theke dekho: Jaspr 1.0 er age syntax bodlay, mone rakha syntax na. Ei README notun README diye bodlao.
2. `jaspr install-skills --agent claude-code` (2.3).
3. `pubspec.yaml` e `shared: {path: ../packages/shared}`; website er nijer resolution (4.6).
4. `.claude/rules/website-jaspr.md` ar 10.x er niyom: dam ba total hishab na, browser Go API te jay na (BFF), token page e third-party script na.
5. Feature doc `docs/features/` e; `.github/workflows/ci.yml` e Dart job.
