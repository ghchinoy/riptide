# Development Note: What a Robust Computer Use Harness Adds (deferred post)

**Status:** DEVELOPMENT NOTE — deferred as a publishable post. Not for publication yet.  
**Source material:** `docs/concepts.md`, `docs/lessons_learned.md`, `docs/validation/`, `docs/validation/efficiency_before_after.md` (pending hyt.6)  
**bd epic:** riptide-r2k; data from riptide-hyt; thesis from riptide-0oo

> **Why this is a development note, not a post (decision 2026-06):**
> The honest value proposition for "why harness design matters" needs comparative data we don't have yet. What we have today is real but insufficient to publish as a developer-facing argument:
> - **Reliability gotchas** — Python reference crashes (`EOFError` on headless CAPTCHA, `BlockReason` SDK break); Riptide handles both. Real, but narrow.
> - **An internal optimization story** — the hyt profiling (we were our own bottleneck). Useful engineering, but it's about fixing our own overhead, not proving harness design value to others.
> - **No demonstrated harness advantage on task outcomes** — on the simple/clean pages tested so far, a robust harness and the flat reference perform about the same. The advantages should appear on complex sites, the benchmark suite, or desktop usage — none of which we've measured yet.
>
> **Publish trigger:** this note becomes a post when at least one of these lands with data:
> 1. `riptide-0oo.6` — tiered vs flat harness measurement on complexity-graded sites
> 2. `riptide-ipo` — benchmark scores (MiniWob++, WebArena) showing harness contribution
> 3. Desktop computer-use tasks (future) — where harness-mediated skills may matter more
>
> Until then: keep this as the working analysis. The migration (Post 1) and sandboxing (Post 4) posts stand on their own and ship without this one.

---

*Google ships a Python reference implementation alongside every computer use model update. It's correct, readable, and a fine starting point. For a tool you run repeatedly on real tasks, the interesting work starts after you outgrow it. This post is about what a more robust harness adds on top of the reference, including the part where we profiled our own harness and discovered we were our own bottleneck.*

---

## What a harness is

A computer use agent has two parts: the model that reasons about what to do, and the harness that executes those decisions in a real browser. The model is Google's. The harness is yours.

A harness has six runtime responsibilities:

| Responsibility | What it does |
|---|---|
| **Observation** | Assembles the per-turn input: screenshot, accessibility tree, DOM state |
| **Context** | Manages what the model sees: history length, pruning, token budget |
| **Control** | Drives the loop: when to call the model, when to stop, what to do on errors |
| **Action** | Translates model output into real browser effects via CDP |
| **State** | Maintains environment state between turns: cookies, URL, session identity |
| **Verification** | Checks that actions had the expected effect before the next turn |

The [computer-use-preview reference agent](https://github.com/google-gemini/computer-use-preview) covers Action (all the tool handlers), Context (screenshot pruning), and Control (the loop with exit conditions). It leaves Verification entirely to the model, State to Playwright's defaults, and Observation to a single screenshot per turn.

Those omissions are reasonable for a reference implementation. They add up in production.

## Getting started: reference vs Riptide

Setting up the Python reference from scratch:

```bash
git clone https://github.com/google-gemini/computer-use-preview
cd computer-use-preview
pip install -r requirements.txt
# For Vertex AI backend:
export USE_VERTEXAI=1
export VERTEXAI_PROJECT=my-project
export VERTEXAI_LOCATION=us-central1
python main.py --query "Navigate to google.com"
```

Five steps before you see a browser move. The env vars aren't validated until the first API call — if you typo `VERTEXAI_PROJECT`, you find out at runtime.

Riptide:

```bash
# Download the binary for your platform, or:
go install github.com/ghchinoy/riptide@latest

riptide config init   # sets up XDG config, validates credentials immediately
riptide run --prompt "Navigate to google.com"
```

`riptide config init` writes to `~/.config/riptide/config.yaml`, validates your GCP credentials before you run a single session, and prints a clear error if something is wrong. No `.env` files, no manual export chains, no surprise at turn one.

Two flags that change the development loop considerably:

- `--show-browser` disables headless mode so you can watch the agent work in a visible Chrome window. When something goes wrong, you see it immediately rather than inferring it from logs.
- `--gif` generates an animated replay of the session from the turn screenshots. A quick way to review what happened without opening the session viewer.

> **[DATA PENDING: riptide-r2k.2]**
> Setup step count and time-to-first-session comparison pending the formal DX audit. The table below will be filled with measured data.
>
> | | Python reference | Riptide |
> |---|---|---|
> | Steps to first session | `[COUNT]` | `[COUNT]` |
> | Credential validation timing | At first API call | At `config init` |
> | Config persistence | `.env` file | XDG YAML |

## The observability gap

When a Python reference session fails, here's what you have: a `print()` statement, a Rich table that scrolled off the terminal, and a `ValueError` traceback if a tool name was wrong.

Riptide emits a typed event for every state transition:

```go
const (
    EventThinking        EventType = "thinking"
    EventStatus          EventType = "status"
    EventSafety          EventType = "safety"
    EventHallucination   EventType = "hallucination"
    EventPromptInjection EventType = "prompt_injection"
    EventError           EventType = "error"
    EventRaw             EventType = "raw"
)
```

Every event goes to two places: the session log file (structured, timestamped, with base64 images truncated to keep it readable) and the `Observer` callback:

```go
// Observer is the single extension point for all session events.
// Plug in your TUI, your test harness, your monitoring system — anything.
type Observer func(Event)
```

The `Observer` is how the TUI, the session viewer, and automated tests all subscribe to the same event stream without touching the agent loop. Pass a different `Observer` and you get a different output surface — same loop, same events.

What this means in practice: when a session ends unexpectedly, you open the session viewer, scrub to the turn where `EventHallucination` fired, and see the exact screenshot the model was looking at when it emitted the bad tool name. The reference sends you to `grep` through terminal output.

> **[DATA PENDING: riptide-r2k.3]**
> The formal observability comparison (session viewer vs Python terminal for the same failed scenario) is pending riptide-r2k.3. This section will be updated with the concrete debugging time comparison.

## The hallucination problem the reference doesn't survive

The model occasionally emits a tool name that doesn't exist in the declared schema. In the Python reference, that path looks like this:

```python
else:
    raise ValueError(f"Unsupported function: {action}")
```

That `ValueError` propagates up, the loop catches it, and the session ends. But the real problem comes one turn earlier. When the reference sends back a `FunctionResponse` with the hallucinated name — `scroll_down`, say — Vertex AI validates that response against the declared tool schema and rejects it:

```
400 Invalid Argument: FunctionResponse.name must match a FunctionCall.name
```

One bad tool name crashes the entire session. All prior context is lost.

Riptide runs a two-layer defense before any tool name reaches the executor:

**Layer 1 — alias mapper.** Common hallucinations get rerouted to the correct tool with the right arguments:

```go
var aliasMap = map[string]string{
    "scroll_down": "scroll",
    "scroll_up":   "scroll",
    "search":      "navigate",
    // ...
}
```

**Layer 2 — history correction.** If the name isn't in the alias map and isn't a registered tool, the offending model turn gets popped off the history before the next API call, and a synthetic correction tells the model what it did wrong:

```go
// Pop the hallucinated content — it must never reach Vertex's validator
history = history[:len(history)-1]

// Inject a correction the model will read on the next turn
history = append(history, &genai.Content{
    Role:  "user",
    Parts: []*genai.Part{{Text: fmt.Sprintf(
        "System Error: '%s' is not a valid tool. Use one of: %s",
        actionName, strings.Join(knownTools, ", "),
    )}},
})
```

The invalid name never reaches Vertex. The session continues. The model corrects itself on the next turn.

> **[DATA PENDING: tier2_results.md]**
> The table below will show measured hallucination recovery rates across the Tier 2 comparison runs (5 tasks × 2 configs × 3 seeds).
>
> | | Riptide | Python reference |
> |---|---|---|
> | Sessions ended by 400 error | `[N]` | `[N]` |
> | Hallucinations intercepted | `[N]` | n/a (crashes) |
> | Sessions recovered after hallucination | `[N]` | `[N]` |

## The safety handler design

The Python reference handles `safety_decision: require_confirmation` with a hardcoded `input()` call:

```python
def _get_safety_confirmation(self, safety):
    print(safety["explanation"])
    decision = input("Do you wish to proceed? [Yes]/[No]\n")
    if decision.lower() in ("n", "no"):
        return "TERMINATE"
    return "CONTINUE"
```

This works for a human at a terminal. It breaks for a test harness, a headless CI run, or any deployment where stdin isn't attached. More importantly, it's not injectable — you can't swap it without modifying the class.

Riptide defines the confirmation contract as a typed function:

```go
// SafetyHandler is called when the model requests require_confirmation.
// Return true to proceed, false to terminate the session.
// If nil, the session terminates automatically — auto-approval is not permitted.
type SafetyHandler func(explanation string) bool
```

Pass a `SafetyHandler` to `computer.Run()` and you control what happens when a safety check fires: show a TUI prompt, auto-deny in headless mode, post to a webhook, answer deterministically in a test. The loop doesn't know or care which — it calls the function and acts on the result.

The nil case is deliberate. We found a bug during our safety audit (riptide-805.9): the original implementation auto-approved when no handler was registered, which violates Google's Terms of Service for computer use. The fix was to make nil mean "terminate," not "continue." Any deployment that wants to approve safety decisions has to register an explicit handler that returns `true`.

## Hybrid tools: grounding the model beyond screenshots

The Python reference uses the `ComputerUse` tool exclusively, with `multiply_numbers` as a placeholder comment showing where you'd add custom functions. In practice, most real deployments need the model to access structured information it can't reliably read from a screenshot.

Riptide registers custom skills alongside the managed `ComputerUse` tool:

```go
// RegisterCustomSkill adds a custom FunctionDeclaration to the tool schema
// alongside the managed ComputerUse tool. The model can call it like any
// other function.
RegisterCustomSkill("get_page_layout", handleGetPageLayout, pageLayoutSchema)
RegisterCustomSkill("get_accessibility_tree", handleGetAXTree, axTreeSchema)
```

The `--axt` flag injects a simplified accessibility tree into the model's context alongside every screenshot. The model gets the DOM's structural description in text — button labels, input types, slider values — without having to infer them from pixels. On pages where visual ambiguity causes coordinate hallucinations (dropdowns that look similar, forms with hidden labels), the accessibility tree gives the model a second, non-visual way to locate elements.

This was a clear win for the 2.5 preview model, whose visual grounding was weaker and which leaned on the semantic tree to hit targets it couldn't reliably locate from pixels. Whether 3.5 Flash, with stronger native grounding, still benefits is an open question we're measuring directly: an A/B ablation with AXT on and off, on tasks built to stress grounding (ambiguous labels, form controls, sliders), not just clean pages where neither model needs the help. The harness should carry the feature only where it pays for its tokens.

## Extension points

The Python reference is a class. To change its behavior, you subclass it or modify it in place. Downstream changes from Google require you to reconcile your fork.

Riptide exposes four injection points that let you change behavior without touching the core loop:

| Extension point | What it controls |
|---|---|
| `SafetyHandler func(string) bool` | User confirmation for `safety_decision` |
| `Observer func(Event)` | Output surface (TUI, logs, tests, monitoring) |
| `RegisterCustomSkill()` | Additional tools in the model's schema |
| `buildSystemInstruction()` | Agent persona and behavioral constraints |

Each is injected at call time. You swap the implementation, not the loop.

## Profiling the harness: we were the bottleneck

The first comparison run was humbling. Against five live tasks at a 10-turn limit, both implementations struggled with complex production SPAs. The interesting finding wasn't the pass rate. It was where the time went.

A typical session's per-turn breakdown looked like this:

| Turn transition | Model call | Gap to next turn |
|---|---|---|
| Navigate, click | 3–5s | ~1s |
| Scroll, then go-back | 4s | **35s** |

The model is fast and consistent. The 15–35 second gaps were our own code. Profiling the loop turned up three sources of avoidable per-turn cost:

1. **A full-page screenshot captured every turn that never reached the model.** The loop measured the full page height, resized the viewport to the entire document (5000px+ on a long SPA), captured a screenshot, and resized back, then wrote it to disk. The result is a debug artifact — useful for a human reviewing a session after the fact, but never sent to the model. On exactly the tall, complex pages that were failing, this resize-capture-resize cycle dominated the per-turn cost, for an image the model never sees.

2. **The accessibility tree fetched and sent on every turn.** Unlike the full-page screenshot, the tree does reach the model. It was a clear win for the weaker 2.5 model. For 3.5, the cost is a CDP round-trip plus the tokens, every turn, including right after a navigation when the model already has a fresh screenshot. Whether 3.5 still needs it is a measurement question (see the AXT ablation above), not an assumed yes.

3. **A coarse "wait for body" between actions.** After a back-navigation, `body` already exists, so the wait returned instantly and the screenshot captured a half-rendered page. The model saw a stale frame, made a poor decision, and the session fell into a scroll-click-go-back loop that burned turns without progress.

None of these are Go versus Python. They're harness design choices, and they were ours to fix.

> **[DATA PENDING: riptide-hyt.6 efficiency re-run]**
>
> The before/after table below will be filled from `docs/validation/efficiency_before_after.md` once the harness efficiency work (epic riptide-hyt) lands: full-page screenshot moved off the hot path, wait strategy tuned per action type, turn limit raised to 20, loop detection added.
>
> | Metric | Before | After |
> |---|---|---|
> | Avg per-turn wall time | `[__s]` | `[__s]` |
> | Harness overhead per turn | `[__s]` | `[__s]` |
> | Avg turns to completion | `[__]` | `[__]` |
> | Task completion rate (5 tasks) | `[__]` | `[__]` |
>
> *Source: re-run Tier 1/Tier 2 via `scripts/run_tier1.sh` and `scripts/run_tier2.sh`, parse with `pkg/results/parser.go`.*

## The harness and the model are one system

The lesson from that profiling run reframed the whole comparison. A computer use agent's behavior is the product of the harness and the model together, not either alone.

The model's quality showed up in the data: zero hallucinated tool names across 18 comparison runs, and a correct refusal to click a CAPTCHA (it emitted `require_confirmation` rather than attempting the interaction). The harness's job is to not waste that quality, by feeding the model fresh frames instead of stale ones, by not spending 35 seconds per turn on a debug artifact, by recovering from a loop instead of burning the turn budget in it.

When the harness feeds the model a clean observation quickly, the model makes good decisions quickly. When the harness feeds it a stale screenshot after an instant non-wait, the model thrashes. Same model, different harness, different outcome.

> **[DATA PENDING: riptide-r2k.1 Python reference comparison]**
>
> The head-to-head comparison data lives in `docs/validation/pub_python_vs_riptide.md`. The two findings that survive regardless of the efficiency tuning:
>
> - **Headless safety handling.** When the CAPTCHA triggered `require_confirmation`, Riptide's headless handler refused and terminated cleanly (TOS-compliant, logged). The Python reference called `input()` against an empty stdin and crashed with `EOFError`. This is the starkest production difference in the comparison.
> - **SDK resilience.** The Python reference crashed on one task with `AttributeError: module 'google.genai.types' has no attribute 'BlockReason'`, a latent break from an SDK update. Riptide's Go SDK integration handled the same safety-block response cleanly.
>
> Browser-level speed is identical (both drive the same Chrome DevTools Protocol). Go's real advantage is concurrency for batch workloads, which matters for benchmark runs and not for single-session latency.

## Start with the reference, then decide

The Python reference is correct. It's what Google tests and maintains. If you're running a demo or exploring what computer use can do, it's the right starting point.

The gap shows up when you run the same task fifty times and need to know which turns failed and why. When you need to swap the safety confirmation handler for a CI environment. When the model hallucinates a tool name and you'd rather recover than crash. When you want to extend the tool schema without forking the agent class.

Riptide's architectural choices — typed interfaces, structured event system, alias mapper, hybrid tool schema — each came from a real failure mode encountered running the reference under production-like conditions. And some of them, like the per-turn overhead we found by profiling, came from auditing our own harness with the same scrutiny we applied to the reference. Building a good harness means tuning it alongside the model, then measuring whether the tuning worked.

A fair word on the language question: this isn't a Go-beats-Python result, and the data wouldn't support that claim. Both drive the same Chrome DevTools Protocol at the same speed; the per-action work is identical. What a robust harness buys you — reliability under failure, structured observability, safe headless behavior, clean extension points — is achievable in either language. We built it in Go for reasons that show up later: compile-time tool contracts as the skill library grows, and cheap concurrency for running multiple page diagnostics at once.

That second thread is where the language choice might actually earn its keep. The reference treats every page the same — one tool, one path, whether it's a static page or a Shadow-DOM-heavy single-page app. The next post takes up whether a harness that adapts to page complexity — activating different skill bundles for different tiers of site, probing the DOM concurrently to decide — measurably outperforms the flat approach. That's where "we built it in Go" becomes a testable claim rather than a preference.

*Riptide is open source at [github.com/ghchinoy/riptide](https://github.com/ghchinoy/riptide). The session log parser that generates comparison data lives in [`pkg/results/parser.go`](https://github.com/ghchinoy/riptide/blob/main/pkg/results/parser.go).*
