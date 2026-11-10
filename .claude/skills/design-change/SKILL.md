---
name: design-change
description: Design (docs/design), schema, migration, pricing/order/phone er niyom, API ba milestone bodlanor kram - age doc, tarpor code ar test. Jokhon-i code design er sathe mile na, notun niyom ba column lagbe, ba user design bodlate bole, tokhon ei skill use koro, code e hat dewar age.
---

Design ei repo er spec: code ke design er dike ana hoy, ulto na. Karon: AI agent ar manush duijon-i design pore kaj kore, tai design bhul thakle porer shob kaj bhul hoy.

1. Bodol ta kon section er, khujo (`AGENTS.md` er map). User ki eta decide koreche? Na korle `docs/design/16-open-decisions.md` er 16.1 e row, `docs/STATUS.md` er "User er kache proshno" e likho ar jiggesh koro; nije decide koro na.
2. Doc age: section er lekha update koro, notun bodole "v1.<n>:" chinho. `docs/design/README.md` e version number barao ar version table e line (tarikh, ki bodlalo, keno); boro decision hole `01-overview.md` er 1.5 decision log e row.
3. Tarpor code ar test. Schema hole: deploy hoya migration (`backend/migrations/DEPLOYED`) edit na, notun `00N_naam.sql` ar `tests/` e test; deploy hoyni emon file hole doc ar file ek shathe.
4. Oi feature er `docs/features/` doc (`feature-doc` skill).
5. `make quick-check` (docsync: file size, code block, link, section map, feature doc).
6. Commit message e section number: `design(8.5): ...`; `docs/STATUS.md` er "Session log" e ek line.
