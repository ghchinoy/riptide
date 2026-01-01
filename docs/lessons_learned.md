# Website Assistant: Lessons Learned

Building a reliable "Computer Use" agent involves solving problems at the intersection of Computer Vision, Browser Automation, and AI Reasoning. Here are the key lessons learned during development.

## 1. Coordinate Drift & Aim Assist
**Problem:** The model's coordinate predictions (0-1000 normalized) often miss the exact pixel target of small elements (buttons, inputs) by 10-20 pixels due to compression artifacts or resolution mismatches.
**Impact:** `MouseClickXY` hits the label next to an input or the padding of a button, failing to trigger focus or action.
**Solution: Euclidean Aim Assist.**
Instead of clicking blindly, we inject JavaScript to:
1.  Check the element at `(x,y)`.
2.  If it's not interactive, scan all inputs/buttons within a **100px radius**.
3.  Calculate the distance to their bounding box centers.
4.  "Snap" the focus/click to the nearest candidate.

## 2. The Focus Gap
**Problem:** Even if `MouseClickXY` hits the input, `chromedp` (and headless browsers in general) can be flaky about setting the `document.activeElement` focus, causing subsequent `KeyEvent` commands to send text to `BODY`.
**Impact:** The model thinks it typed, but the field remains empty.
**Solution: JS Focus Fallback.**
We explicitly call `element.focus()` in our injected JavaScript *before* attempting to type. We then verify focus by checking `document.activeElement.tagName`.

## 3. Button Activation
**Problem:** Simply focusing a `<button>` or `<input type="submit">` does not trigger the click event. `MouseClickXY` might miss the hit target even if Aim Assist finds it.
**Impact:** Agent clicks "Submit" repeatedly, but the form never submits.
**Solution: Auto-Click.**
If the Aim Assist identifies the target as a Button or Submit Input, it forcefully calls `element.click()` in JavaScript to guarantee the action fires.

## 4. Model Hallucination & Visual Feedback
**Problem:** The model sometimes "hallucinates" that it typed text because it *knows* it sent the command, even if the screenshot shows an empty field.
**Impact:** The agent proceeds to the next step prematurely.
**Solution:**
*   **Trust the Screenshot:** We must ensure screenshots are high-quality and taken *after* a sufficient delay (e.g., 1 second) to allow the DOM to update.
*   **Verify via JS:** Future improvement: Use JS to return the *value* of the input to the model in the tool response, so it has "Programmatic" confirmation alongside "Visual" confirmation.

## 5. Safety & CAPTCHAs
**Problem:** Navigating directly to search result URLs (e.g., `google.com/search?q=...`) often triggers anti-bot protections.
**Solution:**
*   **Human-Like Navigation:** Go to the homepage first, then type, then search.
*   **Safety Handling:** The `safety_decision` argument in the API allows the model to signal it's blocked. The application must acknowledge this (or prompt a human) to proceed.

## 6. The Jumpiness Problem (Auto-Scrolling)
**Problem:** Injected `element.focus()` or simple clicks on elements "below the fold" often trigger the browser's default behavior to scroll the element to the top of the viewport.
**Impact:** The page layout jumps, and the model's next turn starts with a screenshot that is visually different from what it expected, leading to confusion or missed clicks.
**Solution: Centered Scrolling.**
When using JS focus assist, we now use `element.scrollIntoView({ behavior: 'instant', block: 'center' })`. This keeps the element in a more predictable position relative to the cursor and reduces drastic layout jumps.

## 7. Shadow DOM & Modern Frameworks
**Problem:** Modern components (like LUTRON or Sonos sliders) often wrap their interactive elements in Shadow Roots. `document.elementFromPoint` only returns the host element, not the internal slider.
**Impact:** Click/Drag hits the "box" but not the "handle".
**Solution: Recursive Deep Element Detection.**
We implemented a `getDeepElement` helper in JS that recursively checks `shadowRoot` until it finds the actual target at the coordinates.

## 8. Semantic Scrolling vs. Pixel Delta
**Problem:** The model often tries to use semantic commands like `scroll_document(direction="down")`.
**Impact:** If the executor only supports `delta_x/y`, these commands result in `0,0` movement.
**Solution: Mapping Directions.**
We added a mapping layer that translates `up/down/left/right` into consistent pixel deltas (e.g., 500px), providing the model with a more intuitive interface.

---

## Appendix: Historical Path to Achievement

This section tracks the evolution of the Website Assistant, documenting the "failed" paths and the pivots that led to our current stability.

### The Viewport Struggle
*   **Path 1: Standard 1024x768.** Initially chosen for compatibility. Resulted in frequent "below the fold" misses where the model could see an element but the executor couldn't reliably interact with it without triggering auto-scrolls.
*   **Path 2: Semantic Scrolling.** Early versions ignored `direction: down`. The agent would "stare" at the fold and repeat the same scroll command indefinitely.
*   **Achievement:** Moved to **1280x1024** as default and implemented directional mapping. This reduced the fold-frequency and gave the model more room to maneuver.

### The Interaction Jump
*   **Path 1: Standard `element.focus()`.** Used to ensure typing went to the right place. Resulted in the browser "jumping" the element to the top of the screen. The model's next turn would start with a completely different visual context, causing it to lose track of its progress.
*   **Achievement:** Pivoted to **Centered Focus** (`scrollIntoView({block: 'center'})`). This stabilized the camera, ensuring the element remained in the middle of the frame after interaction.

### The Shadow DOM Barrier
*   **Path 1: `document.elementFromPoint`.** Standard approach for coordinate-based clicks. Failed on custom LUTRON/Sonos components because they returned the "container" instead of the interactive "handle."
*   **Achievement:** Implemented **Recursive Deep Element Detection**. This allowed the executor to "reach inside" components to find the actual interactive targets.

### TUI Feedback Loop
*   **Path 1: Single Thought String.** The TUI only showed the most recent `thinking` event. Because Gemini often emits multiple thoughts, the final "conclusion" would overwrite the "reasoning," making it hard for users to follow the logic.
*   **Achievement:** Moved to **Thinking History** in the scrollable viewport. Users can now see the full chain of thought alongside the actions.
