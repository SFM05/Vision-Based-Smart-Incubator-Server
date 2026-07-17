#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$ROOT_DIR/bin"
LOG_DIR="$ROOT_DIR/logs"
LISTENER_BIN="$BIN_DIR/listener"
WEB_BIN="$BIN_DIR/web"
LISTENER_PID=""
WEB_PID=""

cd "$ROOT_DIR"

mkdir -p "$BIN_DIR" "$LOG_DIR"

if [[ ! -f ".env" ]]; then
  echo "Warning: .env not found; services will use existing environment variables." >&2
fi

echo "Building backends..."
go build -o "$LISTENER_BIN" ./cmd/server/
go build -o "$WEB_BIN" ./cmd/web/

cleanup() {
  echo "Stopping backends..."

  if [[ -n "$WEB_PID" ]] && kill -0 "$WEB_PID" 2>/dev/null; then
    kill "$WEB_PID"
  fi

  if [[ -n "$LISTENER_PID" ]] && kill -0 "$LISTENER_PID" 2>/dev/null; then
    kill "$LISTENER_PID"
  fi

  wait 2>/dev/null || true
}

trap cleanup INT TERM EXIT

echo "Starting MQTT listener..."
"$LISTENER_BIN" > >(tee -a "$LOG_DIR/listener.log") 2>&1 &
LISTENER_PID=$!

echo "Starting web server on :${WEB_PORT:-8182}..."
"$WEB_BIN" > >(tee -a "$LOG_DIR/web.log") 2>&1 &
WEB_PID=$!

echo "Started backends: listener PID $LISTENER_PID, web PID $WEB_PID"
echo "Logs: $LOG_DIR/listener.log and $LOG_DIR/web.log"
echo "Press Ctrl+C to stop both services."

set +e
wait -n "$LISTENER_PID" "$WEB_PID"
EXIT_CODE=$?
set -e

echo "One backend exited; stopping the other."
exit "$EXIT_CODE"
