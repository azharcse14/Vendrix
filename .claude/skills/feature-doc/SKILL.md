---
name: feature-doc
description: Kono feature er code likha ba bodlanor por, ba user "eta kivabe kaj kore" jiggesh korle, tar doc docs/features/<naam>.md likho ba code er sathe milao (code er map, niyom → test, data, bhul hole ki hoy, proshno-uttor). User `/feature-doc <naam>` diye o dake.
argument-hint: "<feature naam>"
---

# Feature doc: $ARGUMENTS

Feature doc user ke code bujhte ar Claude ke porer session e kaj chaliye jete dey. Tai eta onuman na, code pore likho, ar shudhu ekhon er obostha (itihash git e).

1. `docs/features/README.md` er index e feature ta khojo. Na thakle naam (English, kebab-case, jemon `checkout`), index e row (design section, milestone, obostha), ar code map e notun package.
2. Notun hole `docs/features/_template.md` copy. Heading gulo hubohu, ei kram e (`backend/internal/docsync` dekhe).
3. Code pore likho:
   - "Kivabe kaj kore": request ba job theke database porjonto numbered step, protiti step e file ar function. Dorkar hole choto mermaid (30 line er kom).
   - "Code er map": je kram e porle bujha jay (handler → service → store → test).
   - "Niyom ar tar test": protiti niyom e design section, code er jayga, test er naam hubohu (docsync Go test er naam code e khoje; rename hole doc o).
   - "Data": table, column, migration. "Bhul hole ki hoy": error code ar behavior.
4. Code copy na, link ba naam; 250 line er kom; Banglish (code er naam English).
5. Package er `doc.go` (package comment) e `Feature doc: docs/features/<naam>.md`.
6. User er proshno ar tar uttor "Proshno-uttor" e jog koro.
7. `go test ./internal/docsync` (quick-check er ongsho), tarpor code er sathe ek commit e.
