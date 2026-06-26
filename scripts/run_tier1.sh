#!/usr/bin/env bash
# Tier 1 smoke test runner (riptide-805.2)
# Runs all 5 canonical scenarios and writes session IDs to tier1_results.md.
#
# Prerequisites:
#   make build-agent
#   GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION set (or in ~/.config/riptide/config.yaml)
#
# Usage:
#   ./scripts/run_tier1.sh [--skip-testserver]
set -euo pipefail

RIPTIDE=./bin/riptide
RESULTS=docs/validation/tier1_results.md
SESSIONS_DIR=sessions
MAX_TURNS=15

if [ ! -f "$RIPTIDE" ]; then
  echo "Error: bin/riptide not found. Run 'make build-agent' first." >&2
  exit 1
fi

skip_testserver=false
for arg in "$@"; do
  [ "$arg" = "--skip-testserver" ] && skip_testserver=true
done

run_scenario() {
  local num="$1"
  local prompt="$2"
  echo ""
  echo "=== Scenario $num ==="
  echo "Prompt: $prompt"
  echo ""
  RIPTIDE_NO_TUI=1 "$RIPTIDE" run \
    --prompt "$prompt" \
    --max-turns "$MAX_TURNS" \
    --axt \
    --tui=false \
    --sessions-dir "$SESSIONS_DIR" 2>&1
  # Find the most recent session
  local session_id
  session_id=$(ls -t "$SESSIONS_DIR" 2>/dev/null | head -1)
  echo "Session ID: $session_id"
  echo "$num|$session_id" >> /tmp/tier1_sessions.txt
}

# Clear temp file
: > /tmp/tier1_sessions.txt

run_scenario 1 \
  "Navigate directly to https://flights.google.com and take a screenshot of the landing page."

run_scenario 2 \
  "Go to https://www.google.com and search for 'Gemini 3.5 Flash computer use'. Report the title of the first result you see."

run_scenario 3 \
  "Navigate to the Google Store and find the Pixel phones category. Tell me how many Pixel phone models are currently listed."

run_scenario 4 \
  "Find me a flight from San Francisco (SFO) to Honolulu (HNL) departing next Monday, returning the following Friday. Start at https://flights.google.com and tell me the cheapest round-trip option you find."

if [ "$skip_testserver" = false ]; then
  # Start testserver if not already running
  if ! lsof -i :8080 >/dev/null 2>&1; then
    echo "Starting testserver..."
    go run cmd/testserver/main.go &
    TESTSERVER_PID=$!
    sleep 2
    trap 'kill $TESTSERVER_PID 2>/dev/null' EXIT
  fi

  run_scenario 5 \
    "Go to http://localhost:8080, enter 'Agent Smith' as the name in the form field, and click the Submit button. Tell me what confirmation message appears."
fi

# Build results table
echo ""
echo "=== Results Summary ==="
echo ""
echo "| Scenario | Session ID | Outcome | Turns | Hallucinations |"
echo "|----------|-----------|---------|-------|----------------|"

while IFS='|' read -r num session_id; do
  if [ -n "$session_id" ] && [ -d "$SESSIONS_DIR/$session_id" ]; then
    log="$SESSIONS_DIR/$session_id/session.log"
    outcome=$(grep -oE "Goal Achieved\.|Max Turns Reached\.|Session Finished\." "$log" 2>/dev/null | head -1 || echo "unknown")
    turns=$(grep -oE "Turn [0-9]+/" "$log" 2>/dev/null | tail -1 | grep -oE "[0-9]+" | head -1 || echo "0")
    hallucinations=$(grep -c "\[hallucination\]" "$log" 2>/dev/null || echo "0")
    echo "| $num | $session_id | $outcome | $turns | $hallucinations |"
  fi
done < /tmp/tier1_sessions.txt

echo ""
echo "Full results: $RESULTS"
echo "View sessions: $RIPTIDE sessions list"
