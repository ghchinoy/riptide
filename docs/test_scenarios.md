# Test Scenarios for Computer Use Agents

To validate the robustness of the Website Assistant, we need test environments that target specific interaction capabilities beyond simple form filling.

## 1. The Kanban Board (Drag & Drop)
**Target Capability:** `drag_and_drop`, Coordinate Precision.
**Concept:** A simple Trello-like interface with 3 columns: "ToDo", "In Progress", "Done".
**Task:** "Move the 'Fix Login Bug' card from ToDo to Done."
**Why:**
*   Requires precise start/end coordinates.
*   Tests the `MouseDown` -> `Wait` -> `MouseMove` -> `MouseUp` sequence.
*   Visual verification is easy (card position changes).

## 2. The Mega Menu (Hover)
**Target Capability:** `hover_at`, Timing.
**Concept:** A navigation bar where sub-menus only appear on hover (CSS `:hover` or JS `mouseenter`).
**Task:** "Click on 'Enterprise Solutions' located under the 'Products' menu."
**Why:**
*   The target ('Enterprise Solutions') is *invisible* initially.
*   The agent must infer it needs to hover 'Products' first.
*   Tests if the agent can perform multi-step UI reveals.

## 3. The Infinite Feed (Scroll & Context)
**Target Capability:** `scroll`, Context Pruning.
**Concept:** A social media feed or news site that loads content dynamically as you scroll down.
**Task:** "Find the post by 'User123' about 'Golang' and click 'Like'." (Post is 10 scrolls down).
**Why:**
*   Forces a long session (10+ turns).
*   Validates `MaxRecentScreenshots` pruning: Can the agent remember the goal after 10 turns of scrolling, without crashing the context window?

## 4. The Price Slider (Fine Motor Control)
**Target Capability:** `drag_and_drop` (Horizontal), Coordinate Math.
**Concept:** A flight booking filter with a price range slider ($0 - $1000).
**Task:** "Set the maximum price to $500."
**Why:**
*   Requires mapping abstract values ("$500") to spatial coordinates (pixels).
*   Tests horizontal drag precision.

## 5. The "Safety" Barrier (Human-in-the-Loop)
**Target Capability:** `SafetyHandler`.
**Concept:** A form protected by a mock CAPTCHA or a "Confirm you are human" modal that the model is instructed *not* to solve autonomously (via system instructions) or that requires external validation.
**Task:** "Submit the form."
**Why:**
*   Triggers the `safety_decision` logic.
*   Verifies the CLI prompts the human user for confirmation.

## 6. The Dynamic Shadow DOM (JS Focus Hard Mode)
**Target Capability:** `Smart Focus`, Deep Selectors.
**Concept:** A modern Web Component app using Shadow DOM, where standard `document.elementFromPoint` might return the host component, not the inner input.
**Task:** "Type into the 'Search' box."
**Why:**
*   Stress-tests the "Drill Down" logic in `executor.go`. Does our JS fallback pierce the Shadow DOM? (Likely needs improvement).
