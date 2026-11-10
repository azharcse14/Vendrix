---
name: next
description: User shudhu "next", "continue", "shuru koro", "age barao", "kaj chaliye jao" bole, ba kon kaj korbe sheta na bole kaj chay, tokhon use koro. docs/STATUS.md theke porer kaj niye repo er kram (milestone, fix, design-change, feature-doc) e kore, ar shesh e STATUS update kore.
---

# Porer kaj

User ke "ki korbo?" jiggesh koro na: kaj er list `docs/STATUS.md` e. Karon: user chay Claude Code nijei shob track rakhuk, notun nirdesh chhara.

1. `docs/STATUS.md` puro poro.
   - "Ekhon" e kono kaj adhkhechra thakle (milestone file e tick na deya step) age sheta shesh koro.
   - "User er kache proshno" er kono uttor porer kaj er jonno lagle shudhu oi proshno gulo jiggesh koro. Uttor pele `docs/design/16-open-decisions.md` (16.1) ar dorkari design section e likho (`design-change` skill), STATUS theke proshno ta muche dao.
2. "Porer kaj" er prothom item nao:
   - Milestone er kaj: `milestone` skill er kram, `docs/milestones/M<n>.md` theke. Notun milestone hole plan mode e plan; user approve na kora porjonto code na.
   - Bug ba choto bodol: `fix` skill.
   - Design ba niyom bodol: `design-change` skill.
3. Protiti choto step er por: `make quick-check`, commit (`<type>(<milestone>): ...`), milestone file e tick.
4. Session shesh er age `docs/STATUS.md` (file er upore lekha format): "Ekhon", "Porer kaj" (porer session ekhan theke shuru korbe, tai file, command ar step spashto), "Session log" e ek line. Stop hook eta dekhe.
5. User ke choto report: ki holo, kon test pass, porer kaj, kono proshno.
