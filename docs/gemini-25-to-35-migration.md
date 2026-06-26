# Migrating from Gemini 2.5 Computer Use to Gemini 3.5 Flash

This document captures the concrete changes required to migrate a Go agent harness
from `gemini-2.5-computer-use-preview-10-2025` to `gemini-3.5-flash`, where computer
use is now a built-in native tool rather than a standalone preview model.

It is intended as a reference for a "how we migrated Riptide" write-up.

---

## 1. Model Name

| Before | After |
|--------|-------|
| `gemini-2.5-computer-use-preview-10-2025` | `gemini-3.5-flash` |

The old name is still accepted by the API (listed as a legacy model in the reference
implementation) but receives no further capability updates. Computer use is now a
first-class built-in tool in the main Flash model.

```go
// Before
const ModelName = "gemini-2.5-computer-use-preview-10-2025"

// After
const ModelName = "gemini-3.5-flash"
```

---

## 2. SDK Version

| Before | After |
|--------|-------|
| `google.golang.org/genai v1.39.0` | `google.golang.org/genai v1.62.0` |

v1.62.0 adds new fields to `ComputerUse` that are required for full 3.5 Flash support:
`Environment` and `EnablePromptInjectionDetection`.

---

## 3. `ComputerUse` Tool Configuration

### 3.1 Environment Declaration

`ComputerUse.Environment` is a new field in v1.62.0 that tells the model which kind
of surface it is operating on. For a browser-based harness, always set `EnvironmentBrowser`.

```go
// Before
Tool{ComputerUse: &genai.ComputerUse{}}

// After
Tool{ComputerUse: &genai.ComputerUse{
    Environment: genai.EnvironmentBrowser,
}}
```

### 3.2 Prompt Injection Detection

3.5 Flash includes targeted adversarial training to detect indirect prompt injection.
Enable it via a new boolean field:

```go
ComputerUse: &genai.ComputerUse{
    Environment:                    genai.EnvironmentBrowser,
    EnablePromptInjectionDetection: boolPtr(true),
}
```

When triggered, the model stops generating and sets `FinishReason` to `SAFETY` or
`PROHIBITED_CONTENT`. The harness must detect this and terminate the session:

```go
func isPromptInjectionResponse(cand *genai.Candidate) bool {
    return cand.FinishReason == genai.FinishReasonSafety ||
           cand.FinishReason == genai.FinishReasonProhibitedContent
}
```

---

## 4. `GenerateContentConfig` Additions

### 4.1 SystemInstruction

With 3.5 Flash, Google recommends placing the agent persona and operating constraints
in `SystemInstruction` rather than embedding them in the first user turn alongside
the screenshot:

```go
config := &genai.GenerateContentConfig{
    SystemInstruction: &genai.Content{
        Parts: []*genai.Part{{Text: agentPersonaText}},
    },
    // ... rest of config
}
```

### 4.2 ThinkingConfig

3.5 Flash supports internal chain-of-thought reasoning. Enable it to improve
multi-step planning accuracy on complex tasks:

```go
ThinkingConfig: &genai.ThinkingConfig{
    IncludeThoughts: true,              // Return thought tokens in response
    ThinkingBudget:  int32Ptr(8192),    // Token budget per turn
},
```

`Part.Thought == true` identifies thought tokens in the response. Handle them
separately so they don't get passed to tool execution:

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

---

## 5. Function Call Name Changes

The most impactful migration change: **Gemini 3.5 Flash uses completely different
predefined function call names** from the 2.5 preview. Every name your executor
dispatches on must be updated.

### 5.1 Complete Mapping Table

| Category | 2.5 Preview Name | 3.5 Flash Name | Notes |
|----------|-----------------|----------------|-------|
| **Click** | `click_at` | `click` | Coords still `(x, y)` in 0–1000 range |
| | `left_click` | `click` | Alias removed |
| | `mouse_click` | `click` | Alias removed |
| | `double_click` | `double_click` | Unchanged |
| | `middle_click` | `middle_click` | Unchanged |
| | `right_click` | `right_click` | Unchanged |
| | *(absent)* | `triple_click` | **New** — select-all equivalent on text fields |
| **Mouse press/release** | *(absent)* | `mouse_down` | **New** — press without release |
| | *(absent)* | `mouse_up` | **New** — release without press |
| **Mouse movement** | `hover_at` | `move` | Renamed |
| | `mouse_move` | `move` | Merged |
| **Text input** | `type_text_at(x, y, text)` | `type(text)` | Coords removed; uses active element |
| **Keyboard** | `key_combination(keys)` | `hotkey(keys)` | Renamed |
| | `key` | `press_key` | Renamed (single key) |
| | *(absent)* | `key_down` | **New** — press and hold |
| | *(absent)* | `key_up` | **New** — release held key |
| **Scroll** | `scroll_document(direction)` | `scroll(x, y, direction, magnitude)` | Magnitude in pixels (denormalized) |
| | `scroll_at(x, y, direction)` | `scroll(x, y, direction, magnitude)` | Merged |
| **Wait** | `wait_5_seconds()` | `wait(seconds)` | Duration now explicit |
| **Screenshot** | *(absent)* | `take_screenshot()` | **New** — explicit screenshot action |
| **Navigation** | `go_back` | `go_back` | Unchanged |
| | *(absent)* | `go_forward` | **New** |
| | `navigate(url)` | `navigate(url)` | Unchanged |
| | `search` | *(removed)* | Was already a hallucination alias |

### 5.2 Key Behavioural Differences

**`type` no longer takes coordinates.**  
In 2.5, `type_text_at` required `(x, y)` to click-then-type. In 3.5, `type(text)`
types into the currently active/focused element. The model is expected to click
first to establish focus, then call `type`.

```go
// 2.5 pattern (model emits both in one call)
type_text_at(x=500, y=300, text="hello")

// 3.5 pattern (model emits two separate calls)
click(x=500, y=300)
type(text="hello")
```

**`scroll` now carries explicit pixel magnitude.**  
The old API used a fixed direction with an implicit scroll distance. 3.5 Flash
includes `magnitude` as a pixel value (in 0–1000 normalized range, denormalize the
same way as coordinates):

```go
// 3.5 scroll signature
scroll(x=500, y=400, direction="down", magnitude=400)

// Denormalize magnitude before passing to scrollBy
magnitude_px := magnitude / 1000.0 * viewport_height
```

**`hotkey` replaces `key_combination` for modifier combos.**  
The `keys` argument format is unchanged (`"ctrl+c"`, `"Meta+a"` etc). Only the
function name changed.

### 5.3 Migration Strategy for Executors

The recommended approach (used in Riptide) is to maintain a flat name registry
and register every name — old and new — pointing to the same handler:

```go
// Register both 2.5 and 3.5 names for the same handler
RegisterTool("click", handleMouseClick)    // 3.5
RegisterTool("click_at", handleMouseClick) // 2.5 backward-compat
RegisterTool("left_click", handleMouseClick)

RegisterTool("move", handleMouseMove)      // 3.5
RegisterTool("hover_at", handleMouseMove)  // 2.5

RegisterTool("hotkey", handleKey)          // 3.5
RegisterTool("key_combination", handleKey) // 2.5

RegisterTool("press_key", handleKey)       // 3.5
RegisterTool("key", handleKey)             // 2.5
```

This lets the harness serve both model versions during a transition period without
branching the agent loop.

---

## 6. `type` Argument Changes

| Arg | 2.5 `type_text_at` | 3.5 `type` |
|-----|-------------------|-----------|
| `x` | Required (0–1000) | Absent |
| `y` | Required (0–1000) | Absent |
| `text` | Required | Required |
| `press_enter` | Optional bool | Optional bool |
| `clear_before_typing` | Optional bool | Absent |

If your handler checks for `x`/`y` to decide whether to click first (as Riptide
does), it will silently skip the click for 3.5-style calls — which is correct
behaviour since the model handles the focus separately.

---

## 7. Summary of Harness Config Changes

```go
// Complete GenerateContentConfig for Gemini 3.5 Flash
config := &genai.GenerateContentConfig{
    SystemInstruction: buildSystemInstruction(),      // NEW
    Tools: []*genai.Tool{
        {
            ComputerUse: &genai.ComputerUse{
                Environment:                    genai.EnvironmentBrowser, // NEW
                EnablePromptInjectionDetection: boolPtr(true),            // NEW
            },
        },
    },
    ThinkingConfig: &genai.ThinkingConfig{            // NEW
        IncludeThoughts: true,
        ThinkingBudget:  int32Ptr(8192),
    },
    SafetySettings: []*genai.SafetySetting{ /* unchanged */ },
}
```

And in the response loop, add injection detection before processing tool calls:

```go
if isPromptInjectionResponse(cand) {                  // NEW
    emit(EventPromptInjection, "Injection detected", ...)
    return nil
}
```

---

## 8. What Did Not Change

- The `FunctionResponse` structure (screenshot in `Parts[].InlineData`, URL in `Response`)
- Coordinate normalization (0–1000 range, same denormalization formula)
- Screenshot pruning strategy
- Context history format (`[]*genai.Content` with alternating `user`/`model` roles)
- The `safety_decision` argument handling for human-in-the-loop confirmation
- Hallucination detection via `IsToolKnown()` registry lookup
- The Vertex AI backend and ADC authentication
