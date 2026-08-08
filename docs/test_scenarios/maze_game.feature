Feature: Escape the Mines maze game comprehension and play
  As a validation of Riptide's computer-use harness
  I want to verify Gemini 3.5 Flash can visually understand and play a browser-based maze game
  So that we confirm multi-turn visual reasoning + keyboard input work end-to-end against a real, human-managed browser tab

  Background:
    Given the user has launched Chrome with:
      """
      "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
        --remote-debugging-port=9222 \
        --user-data-dir="$HOME/.riptide-chrome-profile" \
        --no-first-run --no-default-browser-check
      """
    And the user has navigated that Chrome window to "https://www.lotr.com/games/maze"
    And the tab is left open and idle (not yet started)

  Scenario: Discover the open tab
    When I run "riptide targets --cdp-url http://127.0.0.1:9222"
    Then the output should list exactly one "page" target
    And its URL should contain "lotr.com/games/maze"
    And I record its TARGET ID for use in subsequent scenarios

  Scenario: Rules Discovery Probe 1 - Surface Click Behavior
    Given I have the TARGET ID from the discovery scenario
    When I run "riptide run --attach http://127.0.0.1:9222 --tab-id <TARGET_ID> --max-turns 2" with prompt:
      """
      Inspect the maze page. Try clicking on the yellow Run button or directly on the game canvas.
      Report whether a click alone starts the game or moves the player, or if the page state remains unchanged.
      """
    Then the session log should record whether the DOM/screenshot changed after the click

  Scenario: Rules Discovery Probe 2 - Keyboard Start Trigger
    Given the game has not yet started
    When I run "riptide run --attach http://127.0.0.1:9222 --tab-id <TARGET_ID> --max-turns 2" with prompt:
      """
      Use press_key to send 'Enter' or 'Space' to the page.
      Report whether a key press starts the game timer and reveals the player dot and Balrog position.
      """
    Then the DOM content should change from the splash screen ("Into the Dark", "Run") to active game state ("Progress 0%", "Time 0s")

  Scenario: Rules Discovery Probe 3 - Movement Input Options
    Given the game is active
    When I run "riptide run --attach http://127.0.0.1:9222 --tab-id <TARGET_ID> --max-turns 2" with prompt:
      """
      Test pressing 'ArrowRight' or 'w/a/s/d' or clicking the on-screen direction arrows (▲ ◄ ▼ ►).
      Report which input method successfully moves the player dot in the maze.
      """
    Then the session should document which input method altered the player position in the post-action screenshot

  Scenario: Single-step Navigation
    Given the game has been started
    When I run "riptide run --attach http://127.0.0.1:9222 --tab-id <TARGET_ID> --max-turns 3" with prompt:
      """
      Look at the maze. Identify one open direction from the player's current position
      and press that arrow key once. Report whether the player moved and in which direction.
      """
    Then the model should call press_key with one of ArrowUp/ArrowDown/ArrowLeft/ArrowRight
    And the reported direction should match a direction that was visually open
    And the tab should remain open and on the same URL afterward

  Scenario Outline: Multi-turn Maze Solving Attempt
    Given the game is active
    When I run "riptide run --attach http://127.0.0.1:9222 --tab-id <TARGET_ID> --max-turns <turns>" with prompt:
      """
      Solve the maze: navigate the player from its current position to the goal
      using arrow key presses. Move one step at a time, and re-observe the screenshot
      after each move before deciding the next direction. Stop when you reach the goal
      or cannot proceed.
      """
    Then the model should make progress: the player position should change between turns
    And the model should not get stuck in a 3x repeating action loop
    And the final turn's screenshot should show either goal reached or plausible progress

    Examples:
      | turns |
      | 10    |
      | 20    |

  Scenario: Outcome & Constraint Assessment
    Given the Balrog pursuit timer is an active constraint in this game
    When the maze session ends
    Then the model's final response should explicitly acknowledge whether it won, lost, or ran out of turns
    And it should not claim success if the on-screen state does not show a win condition
