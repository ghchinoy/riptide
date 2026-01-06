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

## Shadow DOM Blindness
*   **The Problem:** Standard JS `querySelector` cannot see inside Shadow Roots (common in Web Components like Salesforce or modern UI kits). The agent would see a button in the screenshot but fail to click it.
*   **The Pivot:** We implemented "Deep Hit" detection using `document.elementFromPoint` and recursive shadow root traversal to find the actual interactive node.
*   **Lesson:** The visual layer (Screenshot) sees everything, but the programmatic layer (CDP) is restricted by DOM boundaries. You must bridge this gap.