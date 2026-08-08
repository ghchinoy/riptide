# Post 1: Computer Use Grows Up — Migrating to Gemini 3.5 Flash

**Status:** Draft v2 — editorial pass applied  
**Source material:** `docs/gemini-25-to-35-migration.md`, `docs/lessons_learned.md`  
**Target audience:** Go and Python developers already using Gemini Computer Use  
**Voice:** Medium / dev.to technical  
**Publication dependency:** None — can publish before riptide-805 closes

---

*We built a Go agent harness on Gemini 2.5's computer use preview. When 3.5 Flash landed, every action broke on the first run. This is the map we wish we'd had.*

---

## The first run after the upgrade failed on turn one

I created and maintain [Riptide](https://github.com/ghchinoy/riptide), a Go harness that drives a headless browser using Gemini's computer use capability. The model looks at a screenshot, decides where to click or what to type, and our harness executes that action against a real Chromium instance via the Chrome DevTools Protocol.

The first version targeted `gemini-2.5-computer-use-preview-10-2025`, a standalone preview model dedicated to computer use. When Gemini 3.5 Flash shipped, [computer use stopped being a separate model and became a built-in tool on the mainline Flash model](https://blog.google/innovation-and-ai/models-and-research/gemini-models/introducing-computer-use-gemini-3-5-flash/). That's a real upgrade. It also broke everything.

The first session after we swapped the model name produced a clean API response and a function call our executor had never heard of:

```
unknown action: click
```

The model now emits `click`, not `click_at`. Every action name had changed. This post is the complete map of what changed, what didn't, and how we handled the transition without forking our agent loop in two.

## What actually changed: the short version

The full list:

1. **Model name** — `gemini-2.5-computer-use-preview-10-2025` → `gemini-3.5-flash`
2. **SDK version** — bump to a release that has the new `ComputerUse` fields (we moved to `google.golang.org/genai v1.62.0`)
3. **Config additions** — `Environment`, `EnablePromptInjectionDetection`, `SystemInstruction`, `ThinkingConfig`
4. **Function call names** — a completely different set of predefined names; this is the painful part
5. **Behavioural shifts** — `type` no longer takes coordinates; `scroll` now carries an explicit pixel magnitude

What didn't change: the 0–1000 coordinate system, the `FunctionResponse` structure, the context history format, `safety_decision` handling, and authentication. More on that later.

The model name change is trivial:

```go
// Before
const ModelName = "gemini-2.5-computer-use-preview-10-2025"

// After
const ModelName = "gemini-3.5-flash"
```

The old name still resolves, listed as a legacy model, but receives no further capability updates. Everything interesting is now in the mainline model.

## The function call name problem

This is where a migration that looks like a one-line change becomes a real day of work.

In 2.5, the model emitted actions like `click_at`, `hover_at`, and `type_text_at`. In 3.5 Flash, the entire predefined function set was renamed and restructured. Here's the complete mapping:

| Category | 2.5 Preview | 3.5 Flash | Notes |
|----------|-------------|-----------|-------|
| **Click** | `click_at` | `click` | Coords still `(x, y)`, 0–1000 |
| | `left_click` / `mouse_click` | `click` | Aliases removed |
| | `double_click` | `double_click` | Unchanged |
| | `middle_click` | `middle_click` | Unchanged |
| | `right_click` | `right_click` | Unchanged |
| | *(absent)* | `triple_click` | **New** — select-all on text fields |
| **Mouse press** | *(absent)* | `mouse_down` / `mouse_up` | **New** — press/release without the pair |
| **Movement** | `hover_at` / `mouse_move` | `move` | Renamed and merged |
| **Text** | `type_text_at(x, y, text)` | `type(text)` | **Coords removed** |
| **Keyboard** | `key_combination(keys)` | `hotkey(keys)` | Renamed |
| | `key` | `press_key` | Renamed |
| | *(absent)* | `key_down` / `key_up` | **New** — hold and release |
| **Scroll** | `scroll_document` / `scroll_at` | `scroll(x, y, direction, magnitude)` | Merged; magnitude in pixels |
| **Wait** | `wait_5_seconds()` | `wait(seconds)` | Duration now explicit |
| **Screenshot** | *(absent)* | `take_screenshot()` | **New** — explicit capture |
| **Navigation** | `go_back` | `go_back` | Unchanged |
| | *(absent)* | `go_forward` | **New** |
| | `navigate(url)` | `navigate(url)` | Unchanged |

Two of these are behavioural changes hiding inside name changes, and they'll bite you even after you fix the dispatch table.

**`type` no longer takes coordinates.** In 2.5, `type_text_at(x, y, text)` clicked at a location and then typed. In 3.5, `type(text)` types into whatever element currently has focus. The model is now expected to click first, then type, as two separate actions:

```go
// 2.5: one call did both
type_text_at(x=500, y=300, text="hello")

// 3.5: two calls, model establishes focus first
click(x=500, y=300)
type(text="hello")
```

If your handler used to branch on the presence of `x`/`y` to decide whether to click before typing (ours did), it'll skip the click for 3.5-style calls. That turns out to be exactly right, but only because the model now handles focus separately. Confirm rather than assume.

**`scroll` now carries an explicit pixel magnitude.** The old API scrolled a fixed implicit distance. 3.5 sends a `magnitude` value in the normalized 0–1000 space, which you denormalize the same way you denormalize coordinates:

```go
// 3.5 signature
scroll(x=500, y=400, direction="down", magnitude=400)

// denormalize magnitude against the live viewport before scrolling
magnitudePx := magnitude / 1000.0 * viewportHeight
```

## The migration strategy: register both, branch nothing

The approach that works: **register every name, old and new, pointing to the same handler.**

```go
// Mouse click: 3.5 + 2.5 names, one handler
RegisterTool("click", handleMouseClick)        // 3.5
RegisterTool("click_at", handleMouseClick)     // 2.5
RegisterTool("left_click", handleMouseClick)   // 2.5 alias

// Movement
RegisterTool("move", handleMouseMove)          // 3.5
RegisterTool("hover_at", handleMouseMove)      // 2.5

// Keyboard
RegisterTool("hotkey", handleKey)              // 3.5
RegisterTool("key_combination", handleKey)     // 2.5
RegisterTool("press_key", handleKey)           // 3.5
RegisterTool("key", handleKey)                 // 2.5
```

The same harness serves both model versions without a single `if modelVersion == ...` branch in the agent loop. When you want to A/B the old and new models on the same tasks, this is the difference between one codebase and two. The dispatch table absorbs the difference; the loop never knows.

## New config fields worth enabling

Migration adds capabilities too. 3.5 Flash's config surface gives you capabilities the preview model didn't have.

```go
config := &genai.GenerateContentConfig{
    SystemInstruction: buildSystemInstruction(),       // moved out of user turn
    Tools: []*genai.Tool{{
        ComputerUse: &genai.ComputerUse{
            Environment:                    genai.EnvironmentBrowser,  // NEW
            EnablePromptInjectionDetection: boolPtr(true),             // NEW
        },
    }},
    ThinkingConfig: &genai.ThinkingConfig{             // NEW
        IncludeThoughts: true,
        ThinkingBudget:  int32Ptr(8192),
    },
}
```

**`Environment: EnvironmentBrowser`** tells the model which surface it's driving. Behavior differs by environment. Set it.

**`EnablePromptInjectionDetection: true`** turns on adversarial training that detects when page content is trying to hijack the agent's instructions. When it fires, the model stops generating and sets a safety finish reason. Your loop has to notice and bail:

```go
func isPromptInjectionResponse(cand *genai.Candidate) bool {
    return cand.FinishReason == genai.FinishReasonSafety ||
           cand.FinishReason == genai.FinishReasonProhibitedContent
}
```

Neither the [computer-use-preview agent](https://github.com/google-gemini/computer-use-preview) nor the [Colab tutorial](https://github.com/GoogleCloudPlatform/generative-ai/blob/main/gemini/computer-use/intro_computer_use.ipynb) sets this flag. It defaults to `false`. If you don't opt in, you don't get the protection. Opt in.

**`SystemInstruction`** is where your agent persona and operating constraints now belong, rather than stuffed into the first user turn next to the screenshot. This keeps your constraints out of the conversation history's churn and makes them easier to version.

**`ThinkingConfig`** enables internal chain-of-thought, which measurably helps multi-step planning. The catch: thought tokens come back in the response, and you must not feed them to tool execution. Filter on `Part.Thought`:

```go
for _, part := range cand.Content.Parts {
    if part.Text != "" {
        if part.Thought {
            emit(EventThinking, "[Thinking] "+part.Text, nil)
        } else {
            emit(EventThinking, part.Text, nil)
        }
    }
}
```

## What stayed the same (the gratitude list)

What survived intact:

- **Coordinate normalization** — still 0–1000, same denormalization formula against the live viewport
- **`FunctionResponse` structure** — screenshot in `Parts[].InlineData`, current URL in `Response`
- **Context history** — same `[]*genai.Content` with alternating `user`/`model` roles
- **`safety_decision` handling** — the human-in-the-loop confirmation argument is unchanged
- **Hallucination guard** — our `IsToolKnown()` registry lookup logic carried over directly
- **Vertex AI backend + ADC auth** — no changes

If your 2.5 harness was well-factored, the parts that touched these survived the migration untouched.

## A Go-specific tip: read the module cache, not the docs

The genai Go SDK tracks the Python SDK closely, with Go idioms layered on: `boolPtr()` and `int32Ptr()` helpers, because several new config fields are pointer-to-scalar types so the API can distinguish "unset" from "false."

When you're hunting for which fields a given SDK version actually exposes, the fastest source of truth isn't the docs (which can lag a release). It's the downloaded module source:

```bash
grep -A10 "ComputerUse struct" \
  "$(go env GOPATH)/pkg/mod/google.golang.org/genai@v1.62.0/types.go"
```

That's how we found `EnablePromptInjectionDetection` before it showed up in any guide. And before running a single full session, we pinned the new config in unit tests: assert `EnablePromptInjectionDetection` is non-nil and true, assert `Environment` is `EnvironmentBrowser`. Catches config regressions early.

## The lesson that cost us the most time

We expected the name swaps. The mapping table above took an afternoon. The extra day came from strict validation on the response side.

When the model occasionally emits a name your executor doesn't know (sometimes even a leftover 2.5-style name while running as 3.5), the naive move is to send back a `FunctionResponse` saying "unknown tool." Vertex AI validates that response against the declared tool schema, rejects the unknown name, and crashes the whole session with a `400 Invalid Argument`. One bad action name kills the run.

The fix is to never let an unknown name reach the API. We run a two-layer defense: an alias mapper that re-routes known close-misses (`scroll_down` → `scroll(direction="down")`), and for anything truly unknown, we pop the offending turn off the history and inject a synthetic correction telling the model to try again. The invalid name never reaches Vertex; the model still learns to self-correct.

That defense existed in our 2.5 harness. It mattered more in 3.5, precisely because the model is occasionally inconsistent about old vs new names during the capability's transition period. If you're migrating, build the guard before you need it.

## Migration is the prerequisite, not the destination

Swapping the model name is a one-liner. Making your harness work against 3.5 Flash is a day, most of it in the function name remapping and the response-validation guard, not the config.

The config is where the upside lives. Environment declaration, prompt injection detection, and a thinking budget are real capabilities the preview model never had, and they're the foundation for everything a serious harness does on top of the raw API.

The next post takes up the "on top of the raw API" question: why we think building a Go harness beats using the reference implementation directly. We put Riptide and the Python reference on the same five tasks and measure the difference.

*Riptide is open source at [github.com/ghchinoy/riptide](https://github.com/ghchinoy/riptide). The full migration reference, including every config diff, lives in [`docs/gemini-25-to-35-migration.md`](https://github.com/ghchinoy/riptide/blob/main/docs/gemini-25-to-35-migration.md).*
