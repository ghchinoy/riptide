# Validation Results — Escape the Mines Maze Game

**Date:** 2026-08-07  
**Target:** `https://www.lotr.com/games/maze` (attached via CDP at `http://127.0.0.1:9222`)  
**Model:** `gemini-3.5-flash` (global location)  
**Harness:** Riptide with `RunOptions` attach parameters (`AttachURL`, `TabID`), AXTree, ThinkingConfig (8192 tokens), SystemInstruction, pruning (max 3 screenshots)  

---

## Game Rules & Mechanics Discovered

Through empirical probing with Gemini 3.5 Flash against the live game, we discovered and verified the following rules and mechanics:

| Game Aspect | Discovered Mechanic / Rule | Verification Evidence |
|:---|:---|:---|
| **Game Start Trigger** | **Keyboard (`Enter` / `Space`)**. Clicking the yellow "Run" button on the splash screen does NOT start the game (clicks land on parent `DIV`/`H2` containers around the canvas). Sending `press_key(key="Enter")` immediately transitions the UI from the splash screen to the active game board (`Progress 0%`). | Verified across multiple sessions (`9b60d464`, `e497559e`) — `Enter` keypress launches the game board immediately. |
| **Canvas & Viewport** | The maze renders on an HTML5 `<canvas>` inside a container centered in the viewport (x: 340..640, y: 190..480). | CDP DOM inspection confirmed `CANVAS` element at click coordinates `(627, 343)`. |
| **Player & Goal Markers** | The player is represented by an **orange/yellow dot** at top-left (`~360, 210`). The exit is a **blue dot** at bottom-right (`~626, 462`). | Model reasoning explicitly identified both markers from the initial screenshot on Probe 2/3 and active game sessions. |
| **Movement Controls** | **W3C Keyboard Events (`ArrowUp`, `ArrowDown`, `ArrowLeft`, `ArrowRight`)**. The on-screen direction arrows (▲ ◄ ▼ ►) are non-interactive SVG/DIV decorations; mouse clicks on them land on parent `SECTION` containers and do not move the player. | Upgraded `pkg/computer/tools_standard.go` to dispatch W3C-compliant `KeyDown`/`KeyUp` events with `Key`, `Code` (`"ArrowRight"`), and `WindowsVirtualKeyCode` (`39`/`40`/`37`/`38`). |
| **Verified Movement** | Sending `press_key` with `ArrowDown` and `ArrowRight` moves the player dot along open corridors and updates the in-game state incrementally: `Progress 0% -> 1% -> 3% -> 4%`. | Verified in session logs — `Progress` metrics updated from `0%` to `4%` over consecutive arrow key moves. |
| **Step-based Balrog Pursuit** | The Balrog pursuer advances behind the player as moves are taken (`Balrog 1 behind you` -> `Balrog 2 behind you`). If the player hesitates or hits walls, the Balrog catches up and triggers "The Fire Takes You" overlay. Pressing `Enter` or clicking "Try again" resets the board to `Progress 0%`. | Observed during multi-turn navigation probes — "The Fire Takes You" overlay appears on capture and clears upon pressing `Enter`. |
| **Tab Preservation** | When attaching to a pre-existing target via `WithTargetID`, Riptide detaches cleanly upon session termination by clearing `c.Target.TargetID` prior to context cancellation, preventing `chromedp` from issuing `Target.CloseTarget(id)` on the user's tab. | Verified across 10+ consecutive session runs on Target `30EF9844CF33185DD881764F0112328F` — the tab remained open in Chrome throughout. |

---

## Scenario Execution Summary

| Scenario | Session ID | Outcome | Turns | Total Tokens | Key Discovery |
|:---|:---|:---|:---|:---|:---|
| **Discovery** | `bin/riptide targets` | ✅ `success` | — | — | Found Target `30EF9844CF33185DD881764F0112328F` on `http://127.0.0.1:9222`. |
| **Probe 1 (Clicks)** | `7a6eb46b` | ⚠️ `splash_persisted` | 2 | 9,192 | Click on "Run" button does not start game (`H2`/`DIV` focus result). |
| **Probe 2 (Keyboard Start)** | `9b60d464` | ✅ `game_started` | 2 | 9,442 | `press_key("Enter")` immediately starts the game (`Progress 0%`). |
| **Probe 3 (Key Event Fix)** | `tools_standard.go` | ✅ `fix_applied` | — | — | Implemented W3C key event dispatch (`Key="ArrowRight"`, `Code="ArrowRight"`, `VK=39`). |
| **Probe 4 (Movement Verified)** | `latest` | ✅ `movement_verified` | 10 | 18,200 | Verified player dot movement and DOM progress state updating: `0% -> 1% -> 3% -> 4%`. |

---

## Technical Insights & Fixes Applied

1. **Chromedp `Targets()` Context Bug**: Calling `chromedp.Targets(allocCtx)` directly on a raw allocator context returns `ErrInvalidContext` ("invalid context"). Fixed by creating a proper context via `chromedp.NewContext(allocCtx)` before calling `Targets()`. Added regression test `TestResolveTargetID_NotInvalidContext`.
2. **Tab Preservation Fix**: `chromedp`'s default cancellation behavior on remote contexts is to call `Target.CloseTarget(id)`. Fixed in `pkg/computer/computer.go` by clearing `c.Target.TargetID = ""` before context cancellation when attaching to an existing tab.
3. **W3C Keyboard Event Dispatch Fix**: Standard `chromedp.KeyEvent()` uses `kb.Encode()`, mapping `kb.ArrowRight` to rune `\u0303` without setting `code` or `windowsVirtualKeyCode`. Web canvas game listeners checking `e.key === 'ArrowRight'` or `e.code === 'ArrowRight'` ignored those events. Fixed in `pkg/computer/tools_standard.go` by dispatching full CDP `KeyDown` / `KeyUp` events with W3C DOM attributes (`Key="ArrowRight"`, `Code="ArrowRight"`, `WindowsVirtualKeyCode=39`, `NativeVirtualKeyCode=39`).
4. **CDP Connection Diagnostics**: Switched default CDP URL to `http://127.0.0.1:9222` (avoiding IPv6 `::1` resolution mismatch on macOS where Chrome binds to IPv4 loopback), and improved connection error formatting to report exact dial failures (`connection refused`).
