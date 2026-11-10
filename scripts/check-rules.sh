#!/usr/bin/env bash
# Section 1.4 er je ongsho grep diye dhora jay (heuristic; asol pahara test, review ar DB CHECK).
# Chole: scripts/quick-check.sh (Claude Code Stop hook o) ar CI. Folder na thakle oi check bad.
# Shotti dorkar hole oi line e "// ruleN:ok <karon>" (jemon // rule2:ok contrast ratio); review e dekhabe.
set -u
cd "$(dirname "$0")/.." || exit 1
fail=0
report() { # $1 = ki bhanglo, $2 = grep er output (khali = thik)
  [ -z "$2" ] && return 0
  printf '✗ %s\n' "$1"; printf '%s\n' "$2" | head -n 8 | sed 's/^/    /'
  fail=1
}
existing() { for d in "$@"; do [ -d "$d" ] && printf '%s\n' "$d"; done; }
grep_go() { # $1 = pattern, $2 = ok-tag, baki = folder
  local pat=$1 tag=$2; shift 2
  [ $# -gt 0 ] || return 0
  grep -rnE "$pat" "$@" --include='*.go' | grep -v "// $tag:ok"
}
SQLKW='(SELECT|INSERT|UPDATE|DELETE)[[:space:]]'

# Rule 2: taka BIGINT poisha, float na
# folder er list unquoted (path e space nai); mapfile na, karon macOS er bash 3.2 e nai
money=$(existing backend/internal/pricing backend/internal/order backend/internal/payment \
  backend/internal/coupon backend/internal/cart backend/internal/shipping backend/internal/report)
report "Rule 2: taka-r package e float (poisha int64 rakho)" "$(grep_go 'float(32|64)' rule2 $money)"

# Rule 12: SQL shudhu $n parameter diye
be=$(existing backend)
report "Rule 12: Sprintf ba string jora diye SQL (\$n parameter dao)" "$(
  grep_go "Sprintf\([[:space:]]*\"[^\"]*$SQLKW" rule12 $be
  grep_go "\"[^\"]*${SQLKW}[^\"]*\"[[:space:]]*\+" rule12 $be
  grep_go "\`[^\`]*${SQLKW}[^\`]*\`[[:space:]]*\+" rule12 $be)"

# Rule 10: packages/shared pure Dart
if [ -d packages/shared/lib ]; then
  report "Rule 10: packages/shared e Flutter, dart:io, dart:html ba dart:ui" \
    "$(grep -rnE "^import[[:space:]]+['\"](package:flutter|dart:io|dart:html|dart:ui)" packages/shared/lib)"
fi

# Rule 4: UI te hex rong na (rong theme token theke; preset packages/shared e)
ui=$(existing website/lib admin/lib app/lib)
if [ -n "$ui" ]; then
  report "Rule 4: UI code e hardcoded rong (theme token use koro)" \
    "$(grep -rnE '#[0-9A-Fa-f]{6}([^0-9A-Fa-f]|$)|0x[Ff]{2}[0-9A-Fa-f]{6}' $ui | grep -v '// rule4:ok')"
fi

# 6.1 v1.6: test SQL top-level e thakle embed e migration hishebe chole
report "Migration folder e test SQL (backend/migrations/tests/ e rakho)" \
  "$(ls backend/migrations/*_test.sql 2>/dev/null)"

# 6.1: deploy hoya migration bodlano nishiddho (DEPLOYED er hash mile kina; Bash diye bodlaleo dhora pore)
dep=backend/migrations/DEPLOYED
if [ -f "$dep" ] && grep -qvE '^(#|[[:space:]]*$)' "$dep"; then
  if command -v sha256sum >/dev/null 2>&1; then sum=(sha256sum); else sum=(shasum -a 256); fi
  report "Deploy hoya migration bodleche (6.1): ager moto koro, notun 00N_*.sql e bodol" "$(
    cd backend/migrations && grep -vE '^(#|[[:space:]]*$)' DEPLOYED | "${sum[@]}" -c - 2>&1 | grep -v ': OK$')"
fi

# v1.6: design ekhon docs/design/ e
report "Purono docs/DESIGN.md (design ekhon docs/design/, README.md e suchi)" "$(ls docs/DESIGN.md 2>/dev/null)"

# v1.8: tracking file (AGENTS.md) ar secret file git e na (12.2)
report "docs/STATUS.md nai (Claude Code er tracking, AGENTS.md)" "$([ -f docs/STATUS.md ] || echo docs/STATUS.md)"
if git rev-parse --git-dir >/dev/null 2>&1; then
  report "Secret file git e (.env): git rm --cached koro, secret bodlao" \
    "$(git ls-files | grep -E '(^|/)\.env($|\.)|\.env$' || true)"
fi

[ $fail -eq 0 ] && echo "rule check: ok"
exit $fail
