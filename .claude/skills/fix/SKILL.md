---
name: fix
description: Bug, bhul behavior ba milestone er baire choto bodol thik korar kram - age fail kora test, tarpor fix, feature doc ar STATUS. User bug report korle, "eta thik koro" bolle, ba test fail er karon khujte use koro.
---

# Fix

1. Kon feature (`docs/features/`) ar kon design section. Behavior design er sathe mile na emon kichu bodlate hole `design-change` skill (age doc).
2. Age test: bug ta dhore emon test likho, chaliye fail dekhao.
3. Fix: choto diff, shudhu karon er jaygay; `make quick-check`.
4. Feature doc: "Bhul hole ki hoy" ba "Proshno-uttor" e bug theke shikkha; code er map bodlale map o.
5. Commit `fix(<section>): ...`; `docs/STATUS.md` er "Session log" e ek line (porer kaj bodlale "Porer kaj" o).
