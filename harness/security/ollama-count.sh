#!/usr/bin/env bash
# Counts requests Ollama logged on the host. This is an oracle outside Agenova:
# it sees every request that reached the provider, governed or not.
#
#   ollama-count.sh                 # all requests
#   ollama-count.sh --match e15-x   # only requests whose log line contains e15-x
set -euo pipefail

log="${OLLAMA_SERVER_LOG:-$HOME/.ollama/logs/server.log}"
match=""
while [ $# -gt 0 ]; do
  case "$1" in
    --log) log="$2"; shift 2 ;;
    --match) match="$2"; shift 2 ;;
    *) echo "usage: $0 [--log path] [--match text]" >&2; exit 2 ;;
  esac
done

[ -r "$log" ] || { echo "ollama server log not readable: $log" >&2; exit 1; }

# Docker Desktop forwards container traffic, so every client appears as
# 127.0.0.1; probes are attributed by a unique request path instead.
if [ -n "$match" ]; then
  grep -F '[GIN]' "$log" | grep -cF -- "$match" || true
else
  grep -cF '[GIN]' "$log" || true
fi
