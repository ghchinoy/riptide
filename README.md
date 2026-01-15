# Riptide: The Gemini Computer Use Framework for Go

*A foundational framework for building AI agents that see, reason, and interact with the web.*

![System Architecture](docs/interaction_infographic_v2.png)

## Overview

**Riptide** is a robust reference implementation and framework for the **Gemini 2.5 Computer Use** model. Built in Go, it bridges the gap between Generative AI and the browser, allowing you to build agents that can navigate websites, interact with dynamic content, and process visual information just like a human user.

While usable out-of-the-box as a general-purpose assistant, it is designed to be the **basis for specialized tools**:
*   **Visual QA Testers:** Agents that explore web apps and report visual bugs.
*   **Smart Scrapers:** Extract data from complex, Single-Page Applications (SPAs) where traditional scrapers fail.
*   **Workflow Automation:** Automate repetitive admin tasks, form filling, or "click-ops" workflows.
*   **Screenshot Services:** Intelligent capture tools that navigate to specific states before taking a picture.

## How It Works

The framework implements a continuous **Observe-Reason-Act** loop:

1.  **Observe:** `chromedp` (Headless Chrome) renders the page and captures a high-resolution screenshot.
2.  **Reason:** **Gemini 2.5 (Vertex AI)** analyzes the screenshot and conversation history to decide the next step (e.g., "I need to click the search bar").
3.  **Act:** The `Executor` translates the model's intent into low-level browser events (`MouseClickXY`, `KeyEvent`, `Scroll`).
4.  **Loop:** The result is fed back into the model, allowing for error correction and complex multi-step workflows.

## Prerequisites

*   **Go 1.25+**
*   **Google Cloud Project** with Vertex AI API enabled.
*   **Gemini 2.5 Computer Use Model** access (allowlisted or public preview).
*   **Chrome/Chromium** installed (for `chromedp`).
*   **FFmpeg** (optional, for generating session GIFs).

### Configuration & Environment
Riptide looks for configuration (environment variables) in the following order:
1.  **Actual Environment Variables** already set in your shell.
2.  **Local `.env` file** in the current working directory.
3.  **XDG Config:** `$XDG_CONFIG_HOME/riptide/.env` or `~/.config/riptide/.env`.

**Required Variables:**
```bash
GOOGLE_CLOUD_PROJECT="your-project-id"
GOOGLE_CLOUD_LOCATION="us-central1"
```

## Quick Start

### 1. General Assistant (Now with TUI)
Run the agent with a natural language prompt. By default, it launches a rich **Terminal UI** for real-time monitoring. The `-prompt` flag is **mandatory**.

```bash
go run main.go -prompt "Go to https://google.com and search for 'Gemini Computer Use Go SDK'"
```

### 2. Classic Logging Mode
If you prefer standard stdout logging or are running in a non-interactive environment, disable the TUI:

```bash
go run main.go -prompt "..." -tui=false
```

### 3. Visual Debugging (The "Black Box" Recorder)
Use the `-gif` flag to generate a replay of the agent's session. This is crucial for debugging *why* an agent failed or verifying a test run.

```bash
go run main.go -prompt "..." -gif
```
*Output:* `sessions/<session-uuid>/session.gif`

### 4. Web-Based Session Viewer (New!)
Browse your session history, review agent reasoning, and view high-resolution turn-by-turn screenshot galleries in a beautiful web interface.

**Build and Start:**
```bash
# 1. Build the Lit frontend
(cd frontend && npm install && npm run build)

# 2. Build the Go backend
go build -o session-viewer cmd/session-viewer/main.go

# 3. Start the viewer
./session-viewer
```
*Access:* **`http://localhost:8083`**

### 5. Controlled Testing
The project includes a `testserver` to validate agent behavior against a controlled environment (no internet required).

```bash
# Start the local test bench
go run cmd/testserver/main.go &

# Dispatch the agent
go run main.go -prompt "Go to http://localhost:8080, enter 'Agent Smith' as the name, and click Submit." -gif
```

## Supported Actions

Riptide implements the standard Gemini Computer Use toolset, augmented with advanced heuristics for reliability.

| Action | Source | Description |
| :--- | :--- | :--- |
| `mouse_click` | **Augmented** | Moves cursor and clicks. Employs **Euclidean Aim Assist** to snap to the nearest interactive element if the model's coordinates are slightly off. |
| `type` | **Augmented** | Types text into the active or specified element. Uses **Smart JS Focus** to ensure the target input is ready for characters. |
| `key` | **Native** | Sends individual key presses (e.g., `Enter`, `Escape`) or combinations. |
| `scroll` | **Native** | Scrolls the viewport or specific elements by delta or direction. |
| `drag_and_drop` | **Native** | Performs a complex mouse drag from a start to an end coordinate. |
| `hover` | **Native** | Moves the mouse cursor to a coordinate without clicking (useful for menus). |
| `wait` | **Native** | Pauses execution for a specified duration to allow for async UI updates. |
| `navigate` | **Native** | Directly changes the browser URL. |
| `get_page_layout`| **Riptide** | Scans the DOM and returns a text-based map of interactive elements. Crucial for helping the model "see" when screenshots are ambiguous. |
| `inspect_element`| **Riptide** | Returns the computed CSS styles and ARIA attributes of an element at specific coordinates. |

## Configuration Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-prompt` | (none) | **Mandatory**. The instruction for the agent. |
| `-tui` | `true` | Use the interactive Terminal UI. |
| `-quit-on-exit` | `false` | Automatically exit the TUI when the session finishes. |
| `-gif` | `false` | Generate a `session.gif` replay of the run. |
| `-max-turns` | `10` | Hard limit on the number of turns to prevent runaway costs. |
| `-max-screenshots` | `3` | Number of recent screenshots to keep in history context. Lower values save tokens. |
| `-sessions-dir` | `sessions` | Directory to store session logs and screenshots. |
| `-high-contrast`| `false` | Use a high-contrast theme for the TUI (improves accessibility). |
| `-user-agent` | (Chrome/macOS) | Custom User Agent string to use for the browser session. |
| `-transparent-ua`| `true` | Append Riptide identification to the User Agent (polite mode). |

> **Note:** For more details on how these flags interact and why transparency matters, see the [Identity & Transparency](docs/concepts.md#5-identity--transparency-the-user-agent) section in the Architectural Concepts guide.

## Testing Scenarios
We have documented several test scenarios to validate advanced capabilities like Drag & Drop, Hover, and long-session pruning.
See [Test Scenarios](docs/test_scenarios.md) for details.

## Building Custom Tools

This repository is structured to be extended.

*   **`pkg/computer/computer.go`**: The "Brain". Modify this to change the prompt engineering, history management, or add system instructions.
*   **`pkg/computer/executor.go`**: The "Hands". Extend this to support custom tools (e.g., `extract_data`, `save_file`) that the model can call.
*   **`main.go`**: The "Interface". Wrap this logic into a CLI, HTTP API, or gRPC service for your specific use case.

## Artifacts & Outputs
All run data is organized by **Session UUID** in the configured sessions directory (default `sessions/`):
*   `sessions/<uuid>/session.log`: The full interaction log.
*   `sessions/<uuid>/session.gif`: A full video replay of the task (if enabled).
*   `sessions/<uuid>/screenshots/`:
    *   `turn_N_post.png`: Snapshots taken immediately after every action.
    *   `turn_N_full.png`: Full-page snapshots for debugging.

## Architecture

The system follows a Client-Server-Model pattern:
*   **Client:** The Go application managing the state loop.
*   **Server:** The Browser instance (managed via `chromedp`).
*   **Model:** Vertex AI (Gemini 2.5).

**Core Packages:**
*   `google.golang.org/genai`: The official Go SDK for Gemini.
*   `github.com/chromedp/chromedp`: High-performance Chrome DevTools Protocol client.

**Driver Augmentation:**
To compensate for AI inaccuracy, the Executor employs heuristics like **Euclidean Aim Assist** and **Smart JS Focus** to "snap" clicks to the nearest interactive element.

## Documentation
*   [Architectural Concepts](docs/concepts.md): Deep dive into the "Browser OS" model.
*   [Lessons Learned](docs/lessons_learned.md): Solutions for coordinate drift and focus issues.

## License
Apache 2.0