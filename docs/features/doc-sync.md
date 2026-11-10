# Doc er pahara (docsync)

Design: 4.7, 14.2 test 28 · Milestone: M0 · Code: `backend/internal/docsync`

## Ki kore

`go test ./...` er ongsho, jate doc code er sathe na bhange: design doc er size, code block, link ar section map; feature doc er heading, link, test er naam ar package map; `docs/STATUS.md` ar milestone file er gothon. Stop hook (`quick-check`) eta chalay, tai doc bhule Claude kaj shesh bolte pare na.

## Kivabe kaj kore

1. `parse`: markdown theke heading, code block ar link (code block er bhetore gona hoy na).
2. Design: `TestFileSize` (60 KB), `TestNoLargeCodeBlocks` (80 line), `TestRelativeLinksExist`, `TestSectionsInTheirFile`, `TestIndexes` (README ar AGENTS.md er map).
3. Feature doc: `TestFeatureDocs` (heading er kram, 250 line, link, test er naam `backend` er `*_test.go` e ache kina, index e ache kina).
4. `TestCodeMap`: `backend` er protiti Go package `docs/features/README.md` e; `TestPackageDocs`: protiti package er package comment e feature doc er link, ar sei doc ache.
5. `TestStatusFile`: `docs/STATUS.md` er heading, ar STATUS ar milestone file er link.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/docsync/docsync_test.go` → `parse`, design er test | Design doc |
| `backend/internal/docsync/features_test.go` | Feature doc, code map, package doc, STATUS |
| `backend/internal/docsync/doc.go` | Package comment |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Design file Claude Code er Read e ek bare (4.7) | `TestFileSize` | `TestFileSize` |
| Doc e boro code copy na | `TestNoLargeCodeBlocks` | `TestNoLargeCodeBlocks` |
| Link er file ache | `TestRelativeLinksExist`, `checkLinks` | `TestRelativeLinksExist`, `TestFeatureDocs`, `TestStatusFile` |
| Section thik file e, map thik | `TestSectionsInTheirFile`, `TestIndexes` | `TestSectionsInTheirFile`, `TestIndexes` |
| Feature doc er gothon ar test er naam (4.7) | `TestFeatureDocs` | `TestFeatureDocs` |
| Protiti package er doc ache (4.7) | `TestCodeMap`, `TestPackageDocs` | `TestCodeMap`, `TestPackageDocs` |
| Parser nijei | `parse` | `TestParser` |

## Data

Nai (file pore).

## Bhul hole ki hoy

Test fail e message e file ar ki korte hobe (jemon "README.md er index e ... nai", "... naam er Go test nai").

## Bodlanor age mone rakho

- Notun required heading ba niyom dile `_template.md` ar shob feature doc ek shathe bodlao.
- Test er naam bodlale doc e o (ei test dhore).

## Proshno-uttor

- **Go test e keno, alada script na?** `go test ./...` CI, quick-check ar Stop hook e already chole; alada tool lage na.
