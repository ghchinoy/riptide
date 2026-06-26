# Tier 1 Results — Canonical Smoke Tests

**Date:** 2026-06-26  
**Model:** gemini-3.5-flash (global location)  
**Harness:** Riptide with AXTree, ThinkingConfig (8192 tokens), SystemInstruction, pruning (max 3 screenshots)  
**Config:** `--axt --max-screenshots 3`, headless (RIPTIDE_NO_TUI=1), no interactive safety handler

---

## Results

| # | Scenario | Session ID | Outcome | Turns | Total Tokens | Hallucinations | Safety | Notes |
|---|----------|-----------|---------|-------|-------------|----------------|--------|-------|
| S1 | Direct URL navigation (example.com) | fd9a5889 | ✅ goal_achieved | 2 | 8,672 | 0 | — | Completed in 2 turns. Caching active on turn 2 (3,026 cached). |
| S2 | Google Search | 6e64b084 | ⚠️ safety_denied | 8 | 48,595 | 0 | CAPTCHA | Google displayed CAPTCHA. Model correctly flagged clicking CAPTCHA as `require_confirmation`. Headless handler (no stdin) returned false → terminated. TOS-compliant. |
| S3 | Google Store — Pixel category | 3ed71c0e | ❌ max_turns | 10 | 60,753 | 0 | — | Google Store's reactive JS and navigation complexity exceeded 10-turn limit. |
| S4 | Google Flights — SFO→HNL | bfb90890 | ❌ max_turns | 10 | 63,055 | 0 | — | Flight search form complexity exceeded 10-turn limit. |
| S5 | testserver form fill | 5ad70ae2 | ⚠️ safety_blocked | 5 | 27,787 | 0 | PROMPT_BLOCK | On turn 5: model returned 0 candidates with `blockReason=SAFETY`, message: "The request asks to interact with a local server, which may be a security risk or an unauthorized attempt to access local services." localhost URLs trigger prompt-level safety after ~5 turns. |

**Pass rate: 1/5 (20%)** with 10-turn limit, headless, against live production sites.

---

## Key Findings

### Finding 1 — `gemini-3.5-flash` requires `global` location
`us-central1` returns `404 NOT_FOUND`. All runs used `GOOGLE_CLOUD_LOCATION=global`.

### Finding 2 — Google.com CAPTCHA (S2): Harness safety works correctly
After 8 turns navigating Google Search, Google presented a CAPTCHA. The model correctly called:
```json
{
  "name": "click",
  "args": {
    "safety_decision": {
      "decision": "require_confirmation",
      "explanation": "The action involves interacting with a CAPTCHA verification, which is a restricted interaction according to the Data-2 policy."
    }
  }
}
```
Riptide's headless safety handler (stdin read → EOF → false) correctly terminated without clicking the CAPTCHA. This is TOS-compliant. A TUI session would have prompted the user. **This is a key differentiator from Python reference** — Python's reference impl always prompts; Riptide's headless mode refuses and terminates cleanly.

### Finding 3 — Turn limits too low for S3 and S4
Google Store (S3) and Google Flights (S4) both timed out at 10 turns. These are complex, multi-step reactive SPAs. The reference notebook suggested `max_turns=20`. Recommendation: raise to 20 turns for production use.

Token consumption was high: S3 and S4 used 60k+ tokens, driven by repeated large screenshots of complex UI. AXTree grounding helps but cannot fully compensate for multi-step form flows on modern SPAs.

### Finding 4 — `localhost` triggers prompt-level safety block (S5)
After ~5 turns of interaction with `http://localhost:8080`, the model returned 0 candidates with `blockReason=SAFETY`. The prompt feedback message explicitly cited local server risk. This is a **model-level safety restriction** — not a `safety_decision` arg — so it cannot be overridden by the safety handler.

Mitigation options:
1. Use a publicly-accessible test server (e.g., hosted on Cloud Run) for baseline testing
2. Run Tier 1/S5 interactively with TUI (the safety prompt/rejection appears in the TUI but doesn't block the whole response in that mode — needs further investigation)
3. Accept testserver as CI-only for unit-level executor tests, not full loop smoke tests

### Finding 5 — Caching active and measurable
Turn 2 of S1 showed `cached=3026` tokens. S2 turn 3 showed `cached=4769`. The model is actively caching the static parts of the history. This is a real efficiency gain that will show up in cost comparisons.

---

## Revised Tier 1 Recommendation

For the Python comparison (riptide-r2k.1), use these tasks:

| # | Task | Change from original | Reason |
|---|------|---------------------|--------|
| S1 | example.com heading | Keep | Clean pass, good baseline |
| S2 | Google Search | Keep | CAPTCHA response is a valid finding |
| S3 | Google Store | Raise --max-turns to 20 | Needs more turns for complex SPA |
| S4 | Google Flights | Raise --max-turns to 20 | Same |
| S5 | testserver | Replace with hosted URL or document as "localhost limitation" | Prompt-level safety block |

---

## Token summary

| Scenario | Turns | Total tokens | Tokens/turn avg |
|----------|-------|-------------|-----------------|
| S1 (goal) | 2 | 8,672 | 4,336 |
| S2 (safety) | 8 | 48,595 | 6,074 |
| S3 (max turns) | 10 | 60,753 | 6,075 |
| S4 (max turns) | 10 | 63,055 | 6,306 |
| S5 (blocked) | 5 | 27,787 | 5,557 |
| **Total** | **35** | **208,862** | **5,967 avg** |

Caching becomes significant beyond turn 3, reducing effective prompt token cost by ~50% in steady-state.
