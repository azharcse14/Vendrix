#!/usr/bin/env bash
# Claude Code SessionStart hook: docs/STATUS.md er "Ekhon", "Porer kaj" ar "User er kache proshno" context e
# dey (stdout context e jay), jate notun session e user ke kichu bolte na hoy. Shomossha holeo exit 0.
set -u
root=${CLAUDE_PROJECT_DIR:-$(pwd)}
status="$root/docs/STATUS.md"
echo "Vendrix: CLAUDE.md er kram mano. Kaj er age puro docs/STATUS.md ar oi milestone file poro."
if [ -f "$status" ]; then
  echo
  awk '/^## /{p = ($0 ~ /^## (Ekhon|Porer kaj|User er kache proshno)$/)} p' "$status" | head -n 70
else
  echo "docs/STATUS.md nai: AGENTS.md er Tracking dekhe banao."
fi
if git -C "$root" rev-parse --git-dir >/dev/null 2>&1; then
  echo
  echo "Git: branch $(git -C "$root" branch --show-current 2>/dev/null), uncommitted file $(git -C "$root" status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
fi
exit 0
