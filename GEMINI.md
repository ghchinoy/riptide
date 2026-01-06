# Riptide Coding Conventions

## Project Structure
*   **Root:** `main.go` for entry point CLI logic.
*   **Packages:** `pkg/` for reusable logic.
    *   `pkg/computer`: Core agent logic (`Run`) and tool execution (`Execute`).
*   **Testing:** `cmd/testserver` for a local HTML server to validate browser interactions without external dependencies.

## Task Management (`bd`)
We use the `bd` (Beads) tool for all issue tracking.

*   **Prefix:** `riptide` (or `rt` contextually)
*   **Workflow:**
    1.  **Create:** `bd create "Task Name" --description "Details..."
    2.  **Implement:** Code & Test.
    3.  **Close:** `bd close <id>`
*   **Changelog:**
    To generate `CHANGELOG.md` from closed tasks:
    ```bash
    bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
    ```

## Go Conventions
*   **GenAI SDK:** Use `google.golang.org/genai`.
    *   **Computer Use:** This model uses specific tool definitions (`ComputerUse`). Ensure `FunctionCall` handling is robust.
    *   **Safety:** Always check for and acknowledge `safety_decision` arguments in tool calls.
*   **Chromedp:**
    *   Use a shared context for the session.
    *   Always handle errors (e.g., element not found).
    *   Use `chromedp.Sleep` sparingly; prefer waiting for events/visibility if possible, though Computer Use often requires "blind" interaction based on coordinates.
*   **Logging:**
    *   Log all major turns and tool calls.
    *   **Truncate** large data blobs (like base64 images) in logs using regex to keep output readable.

## Testing & Verification

*   **Unit Tests:** Write tests for `pkg/computer/executor.go` to verify coordinate denormalization and action mapping.

*   **Integration Tests:** Use `cmd/testserver` for reliable, offline testing of form inputs and clicks. This is a first-class citizen for simulating complex DOM behaviors (delayed elements, scrolling).

*   **Visual Logs:** Always support a `-gif` flag to generate visual proof of the session using `ffmpeg`.



## Reliability Patterns

*   **Context Isolation:** Never inherit timeouts from initialization into the main loop. Always create fresh child contexts with specific timeouts for each `chromedp.Run`.
*   **Viewport Stability:** Prefer a larger default viewport (e.g., 1280x1024) to reduce the likelihood of elements being pushed "below the fold" or into complex overflow containers.
*   **Interaction Fallbacks:**
    *   Use "Deep" element detection to penetrate Shadow DOM (found in many modern UI frameworks).
    *   Combine physical mouse events with JS `focus()` and `scrollIntoView({block: "center"})` to ensure the target is ready for interaction.
*   **TUI Event Handling:** Ensure the TUI maintains a history of "thinking" events. Models often emit multiple thoughts per turn, and overwriting a single string leads to loss of reasoning context.

## Lessons Learned

*   **Coordinate Systems:** Gemini Computer Use outputs normalized (0-1000) coordinates. These *must* be denormalized to the browser's viewport size (e.g., 1024x768) before passing to `chromedp`.

*   **Interaction Loop:** The model often expects to see the result of its action. Capturing a screenshot *immediately* after an action (and potentially a short sleep) is critical for the next turn.

*   **Context Management:** Crucial: Do not inherit timeouts from initialization (pre-flight) into the long-lived session context. This leads to `context canceled` errors during API calls.

*   **Observability:** For headless debugging, the TUI must provide toggles for:

    *   **Logs (l):** Real-time session logs.

    *   **JSON (j):** Truncated model responses (truncate base64).

    *   **History (h):** The full request history sent to the model.

*   **Screenshot Reliability:** Initial screenshots can be blank if the renderer hasn't warmed up; add a 1s delay and `WaitReady("body")` checks.