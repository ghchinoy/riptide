# Tier 2 — Comparison Config Definitions

Two configurations used for the 3.5 vs 3.5+Riptide comparison (riptide-805.3).

## Config A — Riptide Enhanced (default)

All harness enhancements active. This is a standard `riptide run` invocation.

```bash
bin/riptide run \
  --prompt "..." \
  --axt                    # AXTree semantic grounding
  --max-turns 15 \
  --max-screenshots 3      # Screenshot pruning active
  # SystemInstruction active (built into computer.go)
  # ThinkingConfig active (8192 token budget)
  # Aim Assist active (Euclidean proximity focus)
  # Hallucination recovery active (IsToolKnown guard)
  # Prompt injection detection active
```

**Harness features active:**
- AXTree injection alongside screenshots
- SystemInstruction with agent persona and safety constraints
- ThinkingConfig (8192 token budget, IncludeThoughts)
- Aim Assist (100px proximity snap to nearest interactive element)
- Hallucination recovery (IsToolKnown guard + correction injection)
- Prompt injection auto-termination
- Screenshot pruning (max 3 in context)

## Config B — Bare 3.5 (harness minimised)

Approximates calling gemini-3.5-flash with no harness augmentation.
Uses Riptide's infrastructure (chromedp, session logging) but disables
all the harness-specific enhancements.

```bash
RIPTIDE_BARE=1 bin/riptide run \
  --prompt "..." \
  --axt=false              # No AXTree
  --max-turns 15 \
  --max-screenshots 1      # Minimal pruning
  # SystemInstruction: generic placeholder only
  # ThinkingConfig: disabled (budget=0)
  # Aim Assist: passthrough click only
  # Hallucination recovery: disabled
```

**Implementation note:** Config B is approximated by setting env var
`RIPTIDE_BARE=1`, which the comparison runner passes as flags. True
"bare 3.5" would require the Python reference; Config B is the closest
achievable within the Go harness infrastructure for a controlled A/B test.

## Metrics captured per run

| Metric | How captured | Source |
|--------|-------------|--------|
| Outcome | `Goal Achieved` / `Max Turns` / `Error` | `pkg/results` |
| Turns | Max turn number in session log | `pkg/results` |
| Hallucinations | Count of `[hallucination]` events | `pkg/results` |
| Actions | All tool calls executed | `pkg/results` |
| Final URL | Last URL in session log | `pkg/results` |
| Wall time | Session directory mtime delta | Comparison runner |

## Task set (5 tasks × 2 configs × 3 seeds = 30 runs)

Tasks chosen to expose harness advantages:

| Task | Targets |
|------|---------|
| T1: Google search + extract result | Baseline navigation |
| T2: Form fill with mis-positioned label | Aim Assist advantage |
| T3: Long scrollable SPA | Scroll + context pruning |
| T4: Page with ambiguous button labels | AXTree grounding advantage |
| T5: testserver multi-step wizard | Hallucination recovery advantage |
