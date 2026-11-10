#!/usr/bin/env bash
# Druto check: protiti kaj shesh e, ar Claude Code er Stop hook eta chalay.
# Go (gofmt, vet, test, docsync) shob shomoy. Dart/Flutter shudhu je folder e pubspec.yaml ache ar bodleche
# (packages/shared bodlale tar upor nirbhor shob app o); QUICK_ALL=1 hole shob (check.sh, CI).
# QUICK_PARTS=go ba dart diye ekta-i (CI er alada job). DB lage emon test TEST_DATABASE_URL chhara skip.
set -euo pipefail
cd "$(dirname "$0")/.."
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.3}"
parts=",${QUICK_PARTS:-go,dart},"

if [[ $parts == *,go,* ]] && [ -d backend ]; then
  (
    cd backend
    unformatted=$("$(go env GOROOT)/bin/gofmt" -l .)
    if [ -n "$unformatted" ]; then printf 'gofmt lagbe:\n%s\n' "$unformatted"; exit 1; fi
    go vet ./...
    go test ./...
  )
fi

changed() { # git e bodla (staged, unstaged, notun) file ache kina; git na thakle shob "bodla"
  git rev-parse --git-dir >/dev/null 2>&1 || return 0
  [ -n "$(git status --porcelain -- "$@" 2>/dev/null)" ]
}
dart_part() { # $1 = folder, $2 = dart | flutter (4.6)
  local dir=$1 tool=$2
  [ -f "$dir/pubspec.yaml" ] || return 0
  if [ "${QUICK_ALL:-0}" != 1 ] && ! changed "$dir" packages/shared; then return 0; fi
  if ! command -v "$tool" >/dev/null 2>&1; then
    if [ -n "${CI:-}" ]; then echo "$dir: CI te $tool nai (Dart job, 4.6)"; return 1; fi
    echo "warning: $dir bodleche kintu $tool nai, check hoyni (CI te hobe)"
    return 0
  fi
  echo "== $dir ($tool)"
  (
    cd "$dir"
    dart format --output=none --set-exit-if-changed .
    "$tool" analyze
    if [ -d test ]; then "$tool" test; fi
  )
}
if [[ $parts == *,dart,* ]]; then
  dart_part packages/shared dart
  dart_part website dart
  dart_part admin flutter
  dart_part app flutter
fi
bash scripts/check-rules.sh
