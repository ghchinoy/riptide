# Riptide: Test Scenarios

To validate the robustness of **Riptide**, we need test environments that target specific interaction capabilities beyond simple form filling.

## Scenario 1: The Infinite Scroll
*   **Target:** A page that loads content as you scroll.
*   **Goal:** Agent must scroll at least 3 times to find a specific "Hidden Treasure" button.
*   **Skill Validated:** `handleScroll` and coordinate persistence during DOM mutation.

## Scenario 2: Shadow DOM Slider
*   **Target:** A custom range input inside a Shadow Root.
*   **Goal:** Agent must move the slider to exactly 75%.
*   **Skill Validated:** "Deep Hit" detection and `drag_and_drop` precision.

## Scenario 3: The Context Window Stress Test
*   **Target:** A multi-step workflow (15+ turns).
*   **Goal:** Complete a complex booking flow without exceeding token limits.
*   **Skill Validated:** Screenshot pruning and history management.

## Scenario 4: The CAPTCHA Barrier
*   **Target:** A login form with a manual safety check.
*   **Goal:** Agent must detect the barrier, pause, and wait for the user to solve it via the TUI.
*   **Skill Validated:** `safety_decision` handling and Human-in-the-Loop flow.