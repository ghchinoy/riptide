# Website Assistant Coding Conventions

## Project Structure
*   **Root:** `main.go` for entry point CLI logic.
*   **Packages:** `pkg/` for reusable logic.
    *   `pkg/computer`: Core agent logic (`Run`) and tool execution (`Execute`).
*   **Testing:** `cmd/testserver` for a local HTML server to validate browser interactions without external dependencies.

## Task Management (`bd`)
We use the `bd` (Beads) tool for all issue tracking.

*   **Prefix:** `website-assistant` (or `wa` contextually)
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
*   **Integration Tests:** Use `cmd/testserver` for reliable, offline testing of form inputs and clicks.
*   **Visual Logs:** Always support a `-gif` flag to generate visual proof of the session using `ffmpeg`.

## Lessons Learned
*   **Coordinate Systems:** Gemini Computer Use outputs normalized (0-1000) coordinates. These *must* be denormalized to the browser's viewport size (e.g., 1024x768) before passing to `chromedp`.
*   **Interaction Loop:** The model often expects to see the result of its action. Capturing a screenshot *immediately* after an action (and potentially a short sleep) is critical for the next turn.
*   **Safety Settings:** The model may trigger "Safety Decisions" (e.g., CAPTCHA detection). The agent must be prepared to handle `safety_decision` arguments in the function call, acknowledging them to proceed or terminate.
*   **Session Management:** Organizing outputs by `session-uuid` allows for parallel runs and easier history tracking.