# Riptide: macOS Desktop Application Testing Extension

This document outlines the architectural design, engineering analysis, and implementation plan for extending **Riptide**—currently a browser-focused agent harness—to support native macOS desktop application testing. 

By leveraging the native **Gemini 3.5 Flash** computer use capability with `ENVIRONMENT_DESKTOP`, we can reuse Riptide's robust Observe-Reason-Act orchestration loop while swapping out browser-level Chrome DevTools Protocol (`chromedp`) interactions for macOS-level OS automation and accessibility query APIs.

---

## 1. Executive Summary & Context

Currently, Riptide is a Go-based agent harness that orchestrates web-browser automation using Gemini 3.5 Flash and `chromedp`. This architecture treats the browser as a "sandboxed operating system." 

Recent updates to the Gemini API (detailed in [computer-use.md.txt](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/computer-use)) formally introduce `ENVIRONMENT_DESKTOP` for Gemini 3.5 Flash. This enables native, coordinate-based visual grounding on a full desktop canvas.

To validate this capability, we use two local sample desktop projects:
1.  **`Elvish`** (`~/projects/elvish-app`): A native macOS app written in Swift and SwiftUI, utilizing standard AppKit/SwiftUI controls.
2.  **`Watcher`** (`~/projects/watcher`): A hybrid macOS application built using Flutter (Dart) with a Go-based background daemon (`watcher-daemon`).

Expanding Riptide to desktop testing requires abstracting Riptide's physical layer. Instead of driving CDP, we must interface directly with macOS APIs to synthesize mouse/keyboard inputs, capture window screenshots, and extract the accessibility tree for semantic grounding.

---

## 2. Gemini 3.5 Flash `ENVIRONMENT_DESKTOP` Toolset

When initialized with `Environment: EnvironmentDesktop`, Gemini 3.5 Flash adjusts its spatial reasoning for a desktop operating system. It relies on a standardized, coordinate-based toolset normalized to a `0-1000` grid for both axes:

| Command Name | Description | Arguments |
|---|---|---|
| `click` | Left-clicks at the coordinate. | `x`, `y` (0-999), `intent` (str) |
| `double_click` | Double-clicks at the coordinate. | `x`, `y` (0-999), `intent` (str) |
| `triple_click` | Triple-clicks at the coordinate. | `x`, `y` (0-999), `intent` (str) |
| `middle_click` | Middle-clicks at the coordinate. | `x`, `y` (0-999), `intent` (str) |
| `right_click` | Right-clicks at the coordinate. | `x`, `y` (0-999), `intent` (str) |
| `mouse_down` | Presses and holds mouse button. | `x`, `y` (0-999), `intent` (str) |
| `mouse_up` | Releases mouse button. | `x`, `y` (0-999), `intent` (str) |
| `move` | Moves cursor to position. | `x`, `y` (0-999), `intent` (str) |
| `type` | Types text into the focused element. | `text` (str), `intent` (str), `press_enter` (bool) |
| `drag_and_drop` | Drags from start to end coordinates. | `start_x`, `start_y`, `end_x`, `end_y`, `intent` (str) |
| `wait` | Suspends execution for N seconds. | `seconds` (int, default 1), `intent` (str) |
| `press_key` | Presses and releases a single key. | `key` (str), `intent` (str) |
| `key_down` / `key_up` | Holds down/releases a specific key. | `key` (str), `intent` (str) |
| `hotkey` | Executes modifier key combinations. | `keys` (list of strings), `intent` (str) |
| `scroll` | Scrolls up/down/left/right by pixels. | `x`, `y` (0-999), `direction`, `magnitude_in_pixels` |
| `take_screenshot` | Native screen/window screenshot. | `intent` (str) |

---

## 3. The 6 Agent Harness Responsibilities on macOS

We map the expansion using the six runtime responsibilities of an agent harness (as defined in `docs/concepts.md`):

```
       ┌─────────────────────────────────────────────────────────┐
       │                     Harness Core                        │
       │               (Observe-Reason-Act Loop)                 │
       └────────────────────────────┬────────────────────────────┘
                                    │
         ┌──────────────────────────┴──────────────────────────┐
         ▼                                                     ▼
┌──────────────────┐                                  ┌──────────────────┐
│  Browser Engine  │                                  │  macOS Desktop   │
│     (Active)     │                                  │   (Extension)    │
├──────────────────┤                                  ├──────────────────┤
│ - chromedp       │                                  │ - CGWindow/Core  │
│ - Chrome AXTree  │                                  │ - AXUIElement    │
│ - JS Injection   │                                  │ - RobotGo/Native │
└──────────────────┘                                  └──────────────────┘
```

### 1. Observation
In a browser, Riptide uses `chromedp.CaptureScreenshot` to capture the viewport, and Chrome's DevTools protocol to fetch the AXTree. On macOS, this translates to:
*   **Visual Capture:**
    *   *Baseline:* Shell out to `/usr/sbin/screencapture -l <window-id> -x output.png` to capture only the targeted application window, or `-c` to capture the clipboard.
    *   *Optimized (Go Native):* Implement CGo bindings to macOS `CoreGraphics` (`CGWindowListCreateImage` or `CGDisplayCreateImage`). This avoids process invocation overhead and allows cropping directly to the targeted window's coordinate bounds.
*   **Accessibility Tree (Semantic Grounding):**
    *   We must query macOS's native **Accessibility API** (`AXUIElementRef`).
    *   By querying the target process ID (PID), we traverse the window hierarchy to extract a tree of elements, their roles (e.g., `AXButton`, `AXTextField`), titles, values, and bounding frames.
    *   This is serialized to JSON and appended to the model history in the same format Riptide currently uses for browser AXTrees, ensuring zero regressions in visual-semantic reasoning.

### 2. Context
*   **Coordinate Translation:** The model generates coordinates on a `0-1000` grid representing its visual field. 
    *   We map this `0-1000` grid directly to the physical pixel bounds of the captured application window (e.g., `1280x800`).
    *   If capturing the full screen, we map to the primary screen dimensions. However, **window-cropped capture** is strongly recommended to isolate the agent from notifications, other open apps, and the macOS menubar.
*   **Window Bounds Resolution:**
    ```go
    // Conceptual calculation
    actualX := windowLeft + (normalizedX / 1000.0) * windowWidth
    actualY := windowTop + (normalizedY / 1000.0) * windowHeight
    ```

### 3. Control
*   **Target Application Lifecycle:**
    *   **Compilation:** Automate building of the target app (e.g., running `make build` or `flutter build macos`).
    *   **Launch:** Start the application process. Retrieve its Process ID (PID) and window ID.
    *   **Focus:** Force the application window to the foreground using AppleScript (`tell application "System Events" to set frontmost of process "..." to true`) or macOS native Cocoa APIs before the first turn.
    *   **Termination:** Send `SIGTERM` (and fallback `SIGKILL`) upon loop completion or abort.
*   **Safety Isolation:** Unlike a sandbox-isolated browser, running a desktop agent has the potential to interact with the host system if the targeted application loses focus. We must enforce active window focus checks on every turn. If the targeted app is no longer the active window, the loop must instantly suspend and alert the user.

### 4. Action
To execute clicks, types, and drags on macOS, Riptide's `Executor` will delegate to a macOS automation library.
*   **Go Driver Selection:** 
    *   `github.com/go-vgo/robotgo`: A cross-platform desktop automation library in Go. It wraps native C/C++ APIs to simulate precise OS-level mouse and keyboard events.
    *   Alternatively, native macOS Cocoa events can be synthesized using CGo to invoke `CGEventCreateMouseEvent` and `CGEventCreateKeyboardEvent`. This is lighter and does not introduce external C-library dependencies.
*   **Aim Assist Adaptations:** Riptide's "Euclidean Aim Assist" currently uses Chrome DOM querying to find nearby clickable nodes. For desktop apps, this will query the extracted macOS AXTree elements instead, computing the closest `AXUIElement` node to snap coordinates.

### 5. State
*   Browser state consists of cookies and local storage. Desktop state is file-based (e.g., user preferences in `~/Library/Application Support/`, SQLite databases, or local configs).
*   The harness must support **state isolation flags**:
    *   `--clean-state`: Temporarily relocates the target app's config directories before run, restoring them upon exit.
    *   `--save-state`: Commits the final application directory state as a snapshot.

### 6. Verification
*   **Focus Check:** Before taking a screenshot, verify that the target PID still holds active window focus via `NSWorkspace.shared.frontmostApplication`.
*   **Crash Detection:** Inspect whether the PID has terminated unexpectedly.

---

## 4. Platform-Specific Target Deep Dives

To implement a test harness, we must address the specific structural differences of our two sample apps:

### App 1: `Elvish` (Native Swift / SwiftUI)
*   **Path:** `/Users/ghchinoy/projects/elvish-app`
*   **Characteristics:** Standard native SwiftUI views compiled into a macOS Cocoa application bundle (`Elvish.app`).
*   **Compilation Command:** `make build` or `xcodebuild -project Elvish.xcodeproj ...`
*   **Accessibility Extraction:** SwiftUI views automatically expose semantic labels to the macOS Accessibility API if they use standard controls (e.g., `Button`, `TextField`, `List`). Custom views should be audited to ensure they use `.accessibilityIdentifier("...")` or `.accessibilityLabel("...")`.
*   **Window Management:** Cocoa windows have standard properties. We can target `Elvish` directly using its bundle identifier (`com.example.Elvish`) to activate, reposition, or measure it.

### App 2: `Watcher` (Flutter macOS App)
*   **Path:** `/Users/ghchinoy/projects/watcher`
*   **Characteristics:** Flutter desktop application (`lib/main.dart`) interacting with a background Go daemon (`watcher-daemon`).
*   **Compilation Command:** `flutter build macos --debug`
*   **Accessibility Extraction:** Flutter uses its own internal render pipeline. To expose elements to the macOS Accessibility API, Flutter builds a native semantic node bridge.
    *   *Requirement:* Ensure the Flutter app is run with Semantics enabled. In Flutter, this can be programmatically forced via `showSemanticsDebugger: true` or automatically triggered when macOS accessibility utilities are querying the app.
    *   *Alternative:* Use Flutter's native integration testing port (`flutter_driver` or `integration_test` package) to run a parallel semantic agent, though native AXTraversal is preferred to test the app in its true compiled production layout.
*   **Daemon Integration:** Since `Watcher` relies on a background Go daemon (`watcher-daemon` in `/Users/ghchinoy/projects/watcher/daemon`), our harness must:
    1.  Start the daemon process first (`./watcher-daemon`).
    2.  Capture daemon log files and expose them to Riptide's `session.log` for integrated diagnostics.
    3.  Gracefully terminate both the daemon and the Flutter UI on exit.

---

## 5. Incremental Engineering & Execution Plan

We propose a 4-phase rollout to integrate macOS desktop testing into Riptide.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ PHASE 1: Native OS Abstraction (Screenshots & Input Synthesis)           │
├──────────────────────────────────────────────────────────────────────────┤
│ PHASE 2: macOS Native AXTree Traversal                                   │
├──────────────────────────────────────────────────────────────────────────┤
│ PHASE 3: Application Lifecycle Controllers (Elvish & Watcher)            │
├──────────────────────────────────────────────────────────────────────────┤
│ PHASE 4: Core Riptide Hookup & Web Viewer Integration                    │
└──────────────────────────────────────────────────────────────────────────┘
```

### Phase 1: Native OS Abstraction (Screenshots & Input Synthesis)
Develop a platform-independent desktop driver interface in Go.

```go
type DesktopDriver interface {
    CaptureWindow(pid int) ([]byte, Rect, error)
    Click(x, y float64, button string) error
    Type(text string, pressEnter bool) error
    SendKey(key string) error
    SendHotkey(keys []string) error
}
```

*   **Implementation:**
    *   Write a native macOS implementation (`desktop_macos.go`) using CGo with standard Cocoa / CoreGraphics frameworks to capture screen rectangles of a target process.
    *   Implement input simulation utilizing CGo calls to AppKit (`CGEventPost`).

### Phase 2: macOS Native AXTree Traversal
Construct an accessibility tree scraper for macOS.
*   Use `ApplicationServices` framework in macOS (`AXUIElementCreateApplication` via CGo).
*   Traverse the UI element tree starting from the application's root window.
*   Output a normalized JSON schema that matches Riptide's browser-based accessibility tree representation:
    ```json
    {
      "role": "AXButton",
      "title": "Submit",
      "value": "",
      "bounds": {"x": 100, "y": 200, "width": 80, "height": 30},
      "children": []
    }
    ```

### Phase 3: Application Lifecycle Controllers
Create specialized execution runners for the target applications.
*   **`ElvishController`**: Handles building of Swift code, launching the app bundle, grabbing window coordinates, and verification.
*   **`WatcherController`**: Handles starting the background Go daemon, spinning up the Flutter macOS app with native semantics enabled, and standardizing their logs.

### Phase 4: Core Riptide Integration & Web Viewer Updates
*   **CLI Flag Additions:**
    *   `-environment=[browser|desktop]` (defaults to `browser`).
    *   `-app-path=<path-to-app-or-bundle>` (e.g., `/Users/ghchinoy/projects/elvish-app/.build/.../Elvish.app`).
    *   `-daemon-path=<path-to-daemon-binary>` (for Watcher).
*   **Brain Config (`pkg/computer/computer.go`):**
    *   Configure `config.Tools` to use `EnvironmentDesktop` when the environment is set to `desktop`.
*   **Session Viewer Enhancements:**
    *   Adapt the Lit-based frontend in `frontend/` to display desktop window metrics instead of web page URLs.
    *   Retain the high-resolution screenshot galleries and reasoning logs, showing OS-level thinking traces.

---

## 6. Code Scaffolding: macOS Desktop Driver

Below is the concrete Go structural scaffolding designed to be integrated into `pkg/computer/`:

```go
// pkg/computer/desktop_macos.go
package computer

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics -framework ApplicationServices
#include <Cocoa/Cocoa.h>
#include <CoreGraphics/CoreGraphics.h>

// Native C helper declarations to capture windows and dispatch events can go here.
*/
import "C"
import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

type MacOSAppController struct {
	PID        int
	BundlePath string
	DaemonPID  int
}

func NewMacOSAppController(bundlePath string) *MacOSAppController {
	return &MacOSAppController{
		BundlePath: bundlePath,
	}
}

// Start launches the target application and brings it to the foreground.
func (c *MacOSAppController) Start(ctx context.Context, daemonPath string) error {
	if daemonPath != "" {
		// Launch background daemon (e.g., watcher-daemon)
		cmdDaemon := exec.CommandContext(ctx, daemonPath)
		if err := cmdDaemon.Start(); err != nil {
			return fmt.Errorf("failed to start background daemon: %w", err)
		}
		c.DaemonPID = cmdDaemon.Process.Pid
	}

	// Launch macOS App Bundle
	cmdApp := exec.Command("open", "-a", c.BundlePath)
	if err := cmdApp.Run(); err != nil {
		return fmt.Errorf("failed to launch macOS app: %w", err)
	}

	// Retrieve PID using bundle identifier or app name
	// In production, we'd query NSWorkspace sharedWorkspace runningApplications
	return nil
}

// EnsureFocus uses AppleScript to bring the target app back to the front.
func (c *MacOSAppController) EnsureFocus(appName string) error {
	script := fmt.Sprintf(`tell application "System Events" to set frontmost of process "%s" to true`, appName)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// Stop terminates all associated processes.
func (c *MacOSAppController) Stop() error {
	if c.PID > 0 {
		syscall.Kill(c.PID, syscall.SIGTERM)
	}
	if c.DaemonPID > 0 {
		syscall.Kill(c.DaemonPID, syscall.SIGTERM)
	}
	return nil
}
```

This scaffolding establishes a robust foundation for building OS-level automation drivers. By executing this plan, Riptide will expand seamlessly into a highly versatile cross-platform agentic testing suite.
