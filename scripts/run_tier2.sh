#!/usr/bin/env bash
# Tier 2 comparison runner (riptide-805.3)
# Runs 5 tasks under Config A (Riptide enhanced) and Config B (bare 3.5),
# each repeated SEEDS times, and generates a comparison report.
#
# Prerequisites:
#   make build-agent
#   GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION set
#   go run cmd/testserver/main.go & (for tasks requiring testserver)
#
# Usage:
#   ./scripts/run_tier2.sh [--seeds N] [--dry-run]
set -euo pipefail

RIPTIDE=./bin/riptide
SESSIONS_DIR=sessions
REPORT=docs/validation/tier2_results.md
SEEDS=3
DRY_RUN=false

for arg in "$@"; do
  case "$arg" in
    --seeds=*) SEEDS="${arg#--seeds=}" ;;
    --dry-run)  DRY_RUN=true ;;
  esac
done

if [ ! -f "$RIPTIDE" ] && [ "$DRY_RUN" = false ]; then
  echo "Error: bin/riptide not found. Run 'make build-agent' first." >&2
  exit 1
fi

# Task definitions: "ID|prompt|max_turns"
declare -a TASKS=(
  "T1|Go to https://www.google.com and search for 'Gemini Flash'. Report the title of the first result.|10"
  "T2|Go to http://localhost:8080 and fill in the name field with 'Validation Test', then submit.|8"
  "T3|Go to https://en.wikipedia.org/wiki/Gemini_(language_model) and scroll to find the section about training data. Report what you find.|12"
  "T4|Go to http://localhost:8080 and click the button labeled 'Submit' without filling in the form first. Report what happens.|6"
  "T5|Go to http://localhost:8080, complete the multi-step form: enter name 'Tier2', select the first option, and submit.|10"
)

# Config definitions: "name|extra_flags"
declare -a CONFIGS=(
  "enhanced|--axt"
  "bare|--axt=false"
)

RESULTS_FILE=/tmp/tier2_results.tsv
echo "config	task	seed	outcome	turns	hallucinations	session_id" > "$RESULTS_FILE"

run_one() {
  local config_name="$1"
  local config_flags="$2"
  local task_id="$3"
  local prompt="$4"
  local max_turns="$5"
  local seed="$6"

  echo "  Running $config_name/$task_id/seed$seed..."

  if [ "$DRY_RUN" = true ]; then
    echo "$config_name	$task_id	$seed	dry_run	0	0	dry-run-session" >> "$RESULTS_FILE"
    return
  fi

  # Run with RIPTIDE_NO_TUI for machine-readable output
  RIPTIDE_NO_TUI=1 "$RIPTIDE" run \
    --prompt "$prompt" \
    --max-turns "$max_turns" \
    --tui=false \
    --sessions-dir "$SESSIONS_DIR" \
    $config_flags \
    >/dev/null 2>&1 || true

  local session_id
  session_id=$(ls -t "$SESSIONS_DIR" 2>/dev/null | head -1)

  if [ -n "$session_id" ] && [ -d "$SESSIONS_DIR/$session_id" ]; then
    local log="$SESSIONS_DIR/$session_id/session.log"
    local outcome turns hallucinations
    outcome=$(grep -oE "Goal Achieved\.|Max Turns Reached\.|prompt_injection|error" "$log" 2>/dev/null | head -1 | tr -d '.' || echo "unknown")
    turns=$(grep -oE "Turn [0-9]+/" "$log" 2>/dev/null | tail -1 | grep -oE "[0-9]+" | head -1 || echo "0")
    hallucinations=$(grep -c "\[hallucination\]" "$log" 2>/dev/null || echo "0")
    echo "$config_name	$task_id	$seed	$outcome	$turns	$hallucinations	$session_id" >> "$RESULTS_FILE"
  fi
}

echo "=== Tier 2 Comparison Run ==="
echo "Tasks: ${#TASKS[@]} | Configs: ${#CONFIGS[@]} | Seeds: $SEEDS"
echo "Total runs: $((${#TASKS[@]} * ${#CONFIGS[@]} * SEEDS))"
[ "$DRY_RUN" = true ] && echo "(DRY RUN — no model calls)"
echo ""

for config_def in "${CONFIGS[@]}"; do
  config_name="${config_def%%|*}"
  config_flags="${config_def##*|}"

  echo "Config: $config_name ($config_flags)"

  for task_def in "${TASKS[@]}"; do
    IFS='|' read -r task_id prompt max_turns <<< "$task_def"
    for seed in $(seq 1 "$SEEDS"); do
      run_one "$config_name" "$config_flags" "$task_id" "$prompt" "$max_turns" "$seed"
    done
  done
done

# Generate report
echo ""
echo "=== Generating Report ==="

python3 - <<'PYEOF'
import sys, os, collections

results_file = "/tmp/tier2_results.tsv"
if not os.path.exists(results_file):
    print("No results file found.")
    sys.exit(0)

rows = []
with open(results_file) as f:
    header = f.readline()
    for line in f:
        parts = line.strip().split("\t")
        if len(parts) >= 6:
            rows.append({
                "config": parts[0], "task": parts[1], "seed": parts[2],
                "outcome": parts[3], "turns": int(parts[4] or 0),
                "hallucinations": int(parts[5] or 0),
            })

if not rows:
    print("No results to report.")
    sys.exit(0)

configs = sorted(set(r["config"] for r in rows))
tasks = sorted(set(r["task"] for r in rows))

print("# Tier 2 Comparison Results")
print("")
print("3.5 Flash (bare) vs 3.5 Flash + Riptide harness")
print("")
print("## Task Completion Rate")
print("")
print(f"| Task | {' | '.join(configs)} |")
print(f"|------|{'|'.join(['---']*len(configs))}|")

for task in tasks:
    row_parts = [task]
    for config in configs:
        task_rows = [r for r in rows if r["task"] == task and r["config"] == config]
        if not task_rows:
            row_parts.append("–")
            continue
        n_complete = sum(1 for r in task_rows if "Goal Achieved" in r["outcome"] or r["outcome"] == "Goal Achieved")
        rate = f"{n_complete}/{len(task_rows)}"
        avg_turns = sum(r["turns"] for r in task_rows) / len(task_rows)
        row_parts.append(f"{rate} ({avg_turns:.1f} turns avg)")
    print("| " + " | ".join(row_parts) + " |")

print("")
print("## Hallucinations Intercepted")
print("")
print(f"| Task | {' | '.join(configs)} |")
print(f"|------|{'|'.join(['---']*len(configs))}|")
for task in tasks:
    row_parts = [task]
    for config in configs:
        task_rows = [r for r in rows if r["task"] == task and r["config"] == config]
        if not task_rows:
            row_parts.append("–")
        else:
            total = sum(r["hallucinations"] for r in task_rows)
            row_parts.append(str(total))
    print("| " + " | ".join(row_parts) + " |")

print("")
print("## Summary")
for config in configs:
    config_rows = [r for r in rows if r["config"] == config]
    n_complete = sum(1 for r in config_rows if "Goal Achieved" in r["outcome"])
    avg_turns = sum(r["turns"] for r in config_rows) / max(len(config_rows), 1)
    total_hallus = sum(r["hallucinations"] for r in config_rows)
    print(f"- **{config}**: {n_complete}/{len(config_rows)} complete, {avg_turns:.1f} turns avg, {total_hallus} hallucinations total")
PYEOF

echo ""
echo "Detailed results: $RESULTS_FILE"
echo "Sessions: $RIPTIDE sessions list"
