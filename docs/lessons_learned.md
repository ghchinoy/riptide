# Riptide: Lessons Learned

This section tracks the evolution of **Riptide**, documenting the "failed" paths and the pivots that led to our current stability.

## Coordinate Drift & Denormalization
*   **The Problem:** Gemini outputs normalized (0-1000) coordinates. Initially, we mapped these directly to `chromedp.MouseClickXY`, but elements were consistently missed because the browser's viewport (e.g., 1280x1024) didn't match the model's aspect ratio or scale.
*   **The Pivot:** We implemented a strict denormalization layer in `executor.go`.
    ```go
    actualX := (normalizedX / 1000.0) * viewportWidth
    actualY := (normalizedY / 1000.0) * viewportHeight
    ```
*   **Lesson:** Never trust raw model output for physical interactions. Always denormalize against the *live* renderer state.

## The "Thinking" Overwrite
*   **The Problem:** In the TUI, the model would often emit several `thinking` blocks followed by a `call`. Because we were only displaying the latest event, the "reasoning" was lost as soon as the action started.
*   **The Pivot:** We updated the TUI to maintain a history of `thinking` events and ensured the final model response is explicitly rendered in the viewport.
*   **Lesson:** AI reasoning is as important as AI action for debugging. Don't let state updates destroy logs.

## Viewport Stability
*   **The Problem:** Many modern sites use `position: fixed` headers or auto-scrolling behaviors. If the agent clicks an element at the bottom of the screen, the browser might scroll, making the coordinates for the *next* action invalid.
*   **The Pivot:** We added `scrollIntoView({block: 'center'})` before every interaction. This ensures the target is always in a predictable visual location for the model's next turn.
*   **Lesson:** Control the environment's physics. If the page moves, the model's "memory" of the screenshot is broken.

## Hallucinated Tools & 400 Invalid Argument Errors
*   **The Problem:** The model would occasionally hallucinate tools that weren't explicitly defined (like `scroll_down`). Our local executor would catch this, generate an error response (`unknown action`), and append it to the context history. When this history was sent back to Vertex AI on the next turn, the API strictly validated the `FunctionResponse` against the defined schema, rejected the unknown tool name, and crashed the entire session with a `400 Invalid Argument`.
*   **The Pivot:** We implemented a two-part defense in `executor.go`:
    1.  **Alias Mapping:** Common hallucinations (`scroll_down`, `search`) are silently intercepted and re-mapped to valid tools (`scroll(direction="down")`, `navigate`) before execution.
    2.  **Safe Rejection:** If a tool is truly unknown, we immediately pop the offending `cand.Content` off the history array and inject a synthetic system prompt (`"Error: You attempted to use an invalid tool..."`). This prevents the invalid tool name from ever reaching the Vertex API validator while still teaching the model to correct its mistake.
*   **Lesson:** When wrapping managed AI APIs, you must vigorously sanitize the conversation history. The LLM might be flexible, but the backend validators are not.