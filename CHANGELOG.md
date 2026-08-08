# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and [Common Changelog](https://common-changelog.org/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `--attach <url>`, `--tab-id <id>`, and `--tab-url-match <substring>` flags on `riptide run` (and corresponding `browser.attach_url`, `browser.tab_id`, `browser.tab_url_match` config keys) enabling Riptide to attach to an existing Chrome browser instance via CDP (`chromedp.NewRemoteAllocator`) instead of spawning a new local browser.
- New `riptide targets --cdp-url <url>` subcommand to discover open tabs and windows in an existing Chrome instance and list their Target IDs and URLs.
- `--model` and `--thinking-budget` flags on `riptide run` to override the model and thinking budget for a single run, following the existing flag > env > config file > default resolution order.
- `model.name` / `model.thinking_budget` config keys (settable via `~/.config/riptide/config.yaml` or `riptide config set`) are now actually wired into `computer.Run()` and used for the `GenerateContent` call. Previously these keys were written/displayed by `riptide config show`/`init` but silently ignored — the model was always the `ModelName` constant regardless of config.
- Warning printed to stderr when `--model`/`model.name` is set to a known legacy computer-use model (e.g. `gemini-2.5-computer-use-preview-10-2025`, `gemini-3-flash-preview`, `gemini-3.1-pro-preview`) that requires a function-call dialect Riptide's harness does not implement. The run still proceeds.
- **Gemini 3.5 Flash** replaces the old `gemini-2.5-computer-use-preview-10-2025` standalone preview model. Computer use is now a built-in native tool in the main Flash model; the reference implementation has moved to `gemini-3.5-flash` as default.
- `SystemInstruction` field in `GenerateContentConfig` separates the agent persona and safety constraints from the user's task prompt, per Gemini 3.5 Flash best practices.
- `ThinkingConfig` with an 8 192-token budget and `IncludeThoughts: true` enables internal chain-of-thought reasoning per turn. Thought tokens are surfaced in the TUI and session logs with a `[Thinking]` prefix.
- `ComputerUse.Environment = EnvironmentBrowser` explicitly declares the operating context to the model (new field in genai SDK v1.62.0).
- `ComputerUse.EnablePromptInjectionDetection = true` activates Gemini 3.5 Flash's adversarial training against indirect prompt injection from page content.
- `isPromptInjectionResponse()` in the agent loop auto-terminates the session when the model signals a detected injection via `FinishReason` `SAFETY` or `PROHIBITED_CONTENT`.
- `EventPromptInjection` event type emitted on automatic prompt injection termination.
- 19 unit tests in `pkg/computer/gemini35_test.go` covering model name, system instruction content, thinking config, helper functions, ComputerUse tool configuration, event uniqueness, and screenshot pruning.
- New hero image `docs/interaction_infographic_v3.webp` generated to reflect the Gemini 3.5 Flash Agent Harness architecture, including `SystemInstruction`, `ThinkingConfig`, and thinking tokens in the flow diagram.

### Changed

- `computer.Run()` now accepts a `computer.RunOptions` struct encapsulating all execution settings (`AttachURL`, `TabID`, `TabURLMatch`, `ModelName`, `ThinkingBudget`, `MaxTurns`, etc.), replacing the positional parameter list.
- `computer.Run()` renames package-level constants to `DefaultModelName`/`DefaultThinkingBudget`, falling back to defaults when options are un-set.
- **genai SDK** upgraded from v1.39.0 to v1.62.0, required for `ComputerUse.Environment`, `EnablePromptInjectionDetection`, and `Part.Thought` fields.
- README reframed around the **agent harness** pattern — the software scaffolding around a foundation model that manages observation, context, control, action, state, and verification.
- `docs/concepts.md` opens with a definition of agent harness (per Kim et al. / Guo et al. 2026) and maps Riptide's components to the six harness responsibilities.
- All "Gemini 2.5 Computer Use" references in README and docs updated to "Gemini 3.5 Flash".
- Prerequisite note updated: Gemini 3.5 Flash is generally available — no allowlist required.

---

## Legacy History

_The entries below predate structured versioning and use the original date/issue format._

### 2026-01-11

#### Added
- Tool Registry pattern in Executor — all tools registered via `RegisterTool` / `RegisterCustomSkill` for runtime lookup and hallucination detection (riptide-b19.1)
- Robust `testserver` for integration testing the executor against a controlled local HTTP server without internet access (riptide-b19.2)
- `.env` configuration loading with XDG-aware fallback (riptide-cvd)

#### Changed
- Unified build and test workflow via `Makefile` (riptide-b19.2)
- Frontend build cleanup and TypeScript configuration fixes (riptide-b19.3)

### 2026-01-14

#### Added
- Accessibility Tree (AXTree) augmentation — simplified Chrome accessibility tree injected alongside screenshots for semantic grounding (`-axt` flag) (riptide-b19)
- Transparent User Agent mode: Riptide appends `(Riptide; +https://github.com/ghchinoy/riptide)` to the UA by default; configurable or suppressible (riptide-igu)
- TUI theme system with `--high-contrast` mode (riptide-9dy)
- XDG configuration conventions (`~/.config/riptide/.env` fallback) (riptide-b19)
- Per-session directory structure: logs and screenshots organised under `sessions/<uuid>/` (riptide-9z0)
- `-quit-on-exit` flag and `-prompt` made mandatory (riptide-b19)
- Session end state reporting: distinguishes "Goal Achieved", "Max Turns Reached", and error exits (riptide-b19)

#### Changed
- `pkg/computer` modularised: tool handlers split into `tools_standard.go` and `tools_augmented.go` (riptide-b19)

### 2026-03-12

#### Added
- Session Viewer packaged as `pkg/viewer` with `--serve` flag on the main binary (riptide-anw)
- WebSocket broadcast hub for live turn-by-turn streaming to the Session Viewer UI (riptide-anw)
- Image lightbox and debug log panel in the Lit frontend (riptide-anw)
- Complete native Computer Use toolset: `right_click`, `middle_click`, `double_click`, `left_click_drag`, `cursor_position`, `hover`, `go_back`, `wait` (riptide-b19)
- Hallucination loop defence: alias mapper (`scroll_down` → `scroll`) and safe rejection that pops the offending history entry and injects a correction prompt to prevent Vertex AI 400 errors (riptide-b19)
- Test suite expanded: TUI state transition tests, session viewer HTTP handler tests, `utils` tests, and full executor integration tests for all mouse button types and cursor tracking

#### Fixed
- Session Viewer: `log.Printf` calls inside `pkg/viewer` were polluting the active `session.log`; replaced with structured writer
- Session Viewer: SPA trailing-slash bug causing directory listing; strict 404 fallthrough for directory paths fixed
- Session Viewer: log and raw event parsing corrected; terminal status events now correctly extracted
- `computer.Run`: guaranteed emission of `Session Finished.` event on all exit paths, including errors
- `cursor_position`: properly tracks and returns last known coordinates via JS injection

### 2026-03-14

#### Added
- Lit frontend WebSocket integration: live turn cards update in real time as `thinking`, `action`, and `status` events arrive

#### Fixed
- Session Viewer: JSON payloads logged and parsed correctly by the web viewer
- Session log bloat: base64 image data truncated in on-disk JSON logs

### 2026-06-07

#### Fixed
- Indentation formatting in `computer.go`

### 2026-06-25

_See the [Unreleased] section above for the full Gemini 3.5 Flash upgrade landed on this date._

[Unreleased]: https://github.com/ghchinoy/riptide/compare/7106090...HEAD
