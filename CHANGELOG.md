# Changelog

## [0.3.0] - 2025-12-21
### Added
- **Terminal UI (TUI):** Launched a rich, interactive Terminal UI built with Bubble Tea.
  - Real-time status updates with spinners.
  - Live "Thinking" and "Action" stream from the Gemini model.
  - Integrated viewport for scrolling through agent logs.
  - Interactive Safety Prompts directly within the TUI (y/n confirmation).
- **Graceful Termination:** Improved TUI handling for `Ctrl+C` and session completion.

## [0.2.0] - 2025-12-15
### Added
- **Reliability:** Implemented "Euclidean Aim Assist" and "Smart JS Focus" to solve coordinate drift and focus loss.
- **Capabilities:** Added `drag_and_drop` and `hover_at` support via `chromedp/cdproto/input`.
- **Safety:** Added interactive `SafetyHandler` for human-in-the-loop confirmation of safety barriers (CAPTCHAs).
- **Optimization:** Added `-max-screenshots` flag to prune old screenshots from history, enabling long-running sessions.
- **Observability:** Added structured events (`thinking`, `action`) and GIF generation.
- **Documentation:** Added `docs/concepts.md`, `docs/lessons_learned.md`, and `docs/test_scenarios.md`.

### Changed
- Refactored `computer.Run` to use the Event Observer pattern.
- Updated `main.go` to handle `Ctrl+C` gracefully.

## [0.1.0] - Initial Release
- Basic Computer Use agent loop.
- `chromedp` integration.
- `click`, `type`, `scroll` tools.
