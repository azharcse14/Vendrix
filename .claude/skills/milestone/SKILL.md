---
name: milestone
description: Section 15 er ekta milestone (M0–M12, MV, MT) shuru theke acceptance porjonto chalay - design pora, plan, age test, implement, feature doc, check, STATUS, report. User `/milestone M3` diye dake, ba `next` skill docs/STATUS.md theke ene dey.
argument-hint: "M<n>"
---

# Milestone $ARGUMENTS

Ei kram e kaj koro, kono step bad diyo na. Karon: acceptance criteria-i ei project er "done" er songgya, ar test age likhle design code e bandha pore.

## 1. Pora
- `docs/STATUS.md`, ar `docs/milestones/$ARGUMENTS.md` thakle seta (ager session er progress ekhane).
- `docs/design/01-overview.md` er Section 1.4.
- `docs/design/15-milestones.md` e $ARGUMENTS er row: Section, Scope, Acceptance criteria.
- Row er Section column er protiti section (file: `AGENTS.md` er map). Scope ba acceptance e onno section ba 14.2 er test number thakle segulo o (`docs/design/14-testing-launch.md`).
- Je feature e hat dibe tar `docs/features/` doc.
- Puro design na. Ager milestone er code shudhu dorkar moto.

## 2. Plan (plan mode)
- [plan-template.md](plan-template.md) diye `docs/milestones/$ARGUMENTS.md` likho (thakle update): scope, file list, protiti acceptance criteria → kon test, 14.2 er test → test er naam, kon feature doc notun ba bodlabe, open question.
- Design e nai emon decision lagle open question e rakho, `docs/design/16-open-decisions.md` (16.1) ar `docs/STATUS.md` er "User er kache proshno" e likho, user ke jiggesh koro. Plan approve na pawa porjonto code na.
- `docs/STATUS.md` er "Ekhon" e milestone ar step.

## 3. Age test
- Business rule (taka, stock, coupon, state, auth, tenant) er test age likho, chaliye fail kore kina dekhao, alada commit e rakho.
- Concurrency ar constraint er test asol Postgres e (C.UTF-8; `scripts/db-test.sh` ba `TEST_DATABASE_URL`); mock DB diye na.

## 4. Implement
- Choto commit, protitir por `make quick-check`.
- Notun Go package e `doc.go` (package comment e `Feature doc: docs/features/<naam>.md`) ar layer `handler.go` → `service.go` → `store.go` (4.2).
- Plan file e checkbox tick, `docs/STATUS.md` er "Ekhon" ar "Porer kaj" update, jate `/clear` ba notun session ekhan theke shuru korte pare.
- Test fail korle code thik koro; test durbol kore pass koriyo na.

## 5. Feature doc
- Protiti notun ba bodla feature e `feature-doc` skill: `docs/features/<naam>.md`, `docs/features/README.md` er index ar code map.

## 6. Check
- `make check` green.
- `design-reviewer` agent ke bodlano file er list ar milestone diye review koro; finding thik koro ba keno thik na seta likho.
- M4 (auth), M10 (payment), ba auth, payment, upload er code bodlale `security-reviewer` agent o.

## 7. Report ar STATUS
- Plan file er shesh e: protiti acceptance criteria → proman (test er naam ba command er output), ki baki, ki design question uthlo.
- `docs/STATUS.md`: Milestone table e obostha, "Ekhon", "Porer kaj" (porer milestone er prothom step), "Session log" e ek line.
- User ke choto report. Merge, push ar deploy user kore.
