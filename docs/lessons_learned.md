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
