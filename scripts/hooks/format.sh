#!/usr/bin/env bash
# Claude Code PostToolUse hook (Edit|MultiEdit|Write): bodla file format kore (Go: gofmt, Dart: dart format).
# Format na hole (syntax bhul) chupchap chere dey; Stop hook er quick-check dhorbe.
set -u
input=$(cat)
if command -v jq >/dev/null 2>&1; then
  f=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
else
  f=$(printf '%s' "$input" | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
fi
[ -n "$f" ] && [ -f "$f" ] || exit 0
root=${CLAUDE_PROJECT_DIR:-$(pwd)}
case "$f" in
  *.go)
    gofmt_bin=""
    if command -v go >/dev/null 2>&1; then gofmt_bin="$(cd "$root/backend" 2>/dev/null && go env GOROOT 2>/dev/null)/bin/gofmt"; fi
    [ -x "$gofmt_bin" ] || gofmt_bin=$(command -v gofmt || true)
    [ -n "$gofmt_bin" ] && "$gofmt_bin" -w "$f" >/dev/null 2>&1 ;;
  *.dart)
    command -v dart >/dev/null 2>&1 && dart format "$f" >/dev/null 2>&1 ;;
esac
exit 0
