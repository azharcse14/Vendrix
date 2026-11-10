@AGENTS.md

## Claude Code: kaj er kram

User ke protibar notun nirdesh dite hobe na: kaj er list, obostha ar niyom repo te. Protibar ei kram mano.

- **Session er shuru**: SessionStart hook `docs/STATUS.md` er "Ekhon", "Porer kaj" ar "User er kache proshno" context e dey. Kaj shuru korar age puro `docs/STATUS.md` ar oi kaj er milestone file (`docs/milestones/M<n>.md`) poro.
- **User shudhu "next", "continue", "shuru koro", "age barao" bolle**: `next` skill. Ki korbe sheta `docs/STATUS.md` e lekha; "ki korbo?" jiggesh korbe na.
- **Milestone**: `milestone` skill (user `/milestone M<n>` diye-o dake). Notun milestone e age plan, user approve korle tarpor code.
- **Bug ba choto bodol**: `fix` skill (age fail kora test).
- **Design, schema ba niyom bodol**: `design-change` skill: age doc, tarpor code ar test.
- **Feature er code likhle ba bodlale**: `feature-doc` skill; doc ar code ek commit e.
- **Kaj shesh bolar age**: `make quick-check` green ar `docs/STATUS.md` update. Stop hook duitai dekhe: code bodleche kintu STATUS bodlay nai hole atkay. Milestone shesh e `make check` ar `design-reviewer` agent; auth, payment ba upload hole `security-reviewer` o.
- **Commit**: tumi koro, choto (ek commit e ek kaj), message `<type>(<milestone ba section>): ...` (jemon `feat(M0): pgx driver`, `fix(8.6): ...`, `docs(features): ...`). `git push`, merge ar deploy user kore.
- **Test** durbol kore (case muche, golden bodle, skip kore) pass koriyo na; test bodlate hole karon ar section bolo.
- **Hook** kono kaj atkale (`BLOCKED:`) bypass koro na; karon user ke bolo. `.claude/settings.json`, `scripts/hooks/` ar `backend/migrations/DEPLOYED` shudhu user (ba deploy job) bodlay.
- **Proshno**: design e nai emon decision lagle `docs/design/16-open-decisions.md` (16.1) e row ar `docs/STATUS.md` er "User er kache proshno" e likho, tarpor user ke jiggesh koro. Uttor na pawa porjonto oi kaj na; onno kaj thakle sheta koro.
- **Bhasha**: user er sathe ar doc e Banglish, code er naam English. Code comment o Banglish, choto, shudhu "keno"; itihash (`v1.x:`) notun code e na, git ar doc e.
