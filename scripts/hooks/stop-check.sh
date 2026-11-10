#!/usr/bin/env bash
# Claude Code Stop hook: Claude "shesh" bolar age:
#   1. scripts/quick-check.sh (gofmt, vet, test, docsync, rule; Dart bodla hole)
#   2. code bodleche (uncommitted, ba docs/STATUS.md er shesh commit er por commit) kintu STATUS update hoyni
# thik → exit 0; na → exit 2 (stderr Claude pay ar kaj chaliye jay); ek turn e 3 bar er por exit 1 (warning
# user dekhe, Claude thame; infinite loop na).
set -u
input=$(cat)
root=${CLAUDE_PROJECT_DIR:-$(pwd)}
cd "$root" 2>/dev/null || exit 0
[ -f scripts/quick-check.sh ] || exit 0

field() { # $1 = JSON key; jq na thakle sed
  if command -v jq >/dev/null 2>&1; then printf '%s' "$input" | jq -r --arg k "$1" '.[$k] // empty'
  else printf '%s' "$input" | sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n 1; fi
}
sid=$(field session_id | tr -cd 'A-Za-z0-9_-')
state="${TMPDIR:-/tmp}/vendrix-stop-${sid:-nosession}"

problem=""
if ! out=$(bash scripts/quick-check.sh 2>&1); then
  problem="scripts/quick-check.sh fail. Kaj shesh bolar age thik koro; test durbol kore pass koriyo na:
$(printf '%s\n' "$out" | tail -n 40)"
elif git rev-parse --git-dir >/dev/null 2>&1; then
  code=(backend packages website admin app infra scripts .github Makefile)
  dirty_code=$(git status --porcelain -- "${code[@]}" 2>/dev/null | head -n 1)
  dirty_status=$(git status --porcelain -- docs/STATUS.md 2>/dev/null | head -n 1)
  code_ts=$(git log -1 --format=%ct -- "${code[@]}" 2>/dev/null)
  status_ts=$(git log -1 --format=%ct -- docs/STATUS.md 2>/dev/null)
  if [ -z "$dirty_status" ] && { [ -n "$dirty_code" ] || [ "${code_ts:-0}" -gt "${status_ts:-0}" ]; }; then
    problem="Code bodleche kintu docs/STATUS.md update hoyni. Shesh bolar age \"Ekhon\", \"Porer kaj\" ar \"Session log\" update koro (file er upore format), milestone file ar feature doc o milao."
  fi
fi
if [ -z "$problem" ]; then
  rm -f "$state"
  exit 0
fi
n=$(cat "$state" 2>/dev/null || echo 0)
case $n in '' | *[!0-9]*) n=0 ;; esac
if [ "$n" -ge 3 ]; then
  rm -f "$state"
  { echo "Stop check 3 bar fix er cheshtar por-o fail. Nije dekho:"; printf '%s\n' "$problem" | tail -n 25; } >&2
  exit 1
fi
echo $((n + 1)) >"$state"
printf '%s\n' "$problem" >&2
exit 2
