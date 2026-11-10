#!/usr/bin/env bash
# Claude Code PreToolUse hook (Edit|MultiEdit|Write): je file agent bodlate parbe na, segulo atkay.
# exit 2 = block, stderr Claude pay. Bash diye bodlano CI te dhora pore (DEPLOYED er hash, check-rules.sh).
set -u
input=$(cat)
if command -v jq >/dev/null 2>&1; then
  f=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
else
  f=$(printf '%s' "$input" | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
fi
[ -n "$f" ] || exit 0
root=${CLAUDE_PROJECT_DIR:-$(pwd)}
rel=${f#"$root"/}
block() { echo "BLOCKED: $1" >&2; exit 2; }

case "$rel" in
  .claude/settings.json | .claude/settings.local.json | scripts/hooks/*)
    block "$rel Claude Code er permission ba hook: shudhu user bodlay. Ki bodlate hobe ar keno, user ke bolo." ;;
  .env | .env.* | */.env | */.env.* | *.env)
    block "secret file ($rel) agent bodlay na; udahoron backend/env.example e." ;;
  backend/migrations/DEPLOYED)
    block "DEPLOYED shudhu production deploy er por deploy job ba user bodlay (13.6)." ;;
  backend/migrations/tests/*) ;;
  backend/migrations/*_test.sql)
    block "test SQL shudhu backend/migrations/tests/ e: top-level e thakle embed e migration hishebe chole (6.1)." ;;
  backend/migrations/*.sql)
    name=${rel#backend/migrations/}
    if [ -f "$root/backend/migrations/DEPLOYED" ] &&
       awk -v n="$name" '!/^#/ && ($2 == n || $2 == "*" n) { found = 1 } END { exit !found }' "$root/backend/migrations/DEPLOYED"; then
      block "$name production e deploy hoyeche, edit nishiddho (6.1). Notun backend/migrations/00N_naam.sql banao."
    fi ;;
  docs/DESIGN.md)
    block "design docs/design/ e (README.md e suchi); ek file er DESIGN.md abar banabe na." ;;
esac
exit 0
