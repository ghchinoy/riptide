# Riptide: Architectural Concepts

## What Is an Agent Harness?

Recent literature (Kim et al. 2026; Guo et al. 2026; Zheng et al. 2026) has converged on the term **agent harness** to describe the software scaffolding that sits around a foundation model in a deployed agent system. The harness is distinct from the model itself: it determines which tools the model can call, how observations are assembled and formatted, how context is managed across turns, how actions are executed in the environment, and how safety policies are enforced.

Concretely, a harness decomposes into six coupled runtime responsibilities:

| Responsibility | What it does |
| :--- | :--- |
| **Observation** | Assembles the per-turn input — screenshots, accessibility trees, DOM state |
| **Context** | Manages what the model sees: history length, pruning, token budget |
| **Control** | Drives the loop — when to call the model, when to stop, what to do on errors |
| **Action** | Translates model output into real environment effects |
| **State** | Maintains environment state between turns — cookies, URL, session identity |
| **Verification** | Checks that actions had the expected effect before the next turn |

**Riptide is a Go implementation of this pattern for Gemini 3.5 Flash's built-in browser computer use tool.**

## Riptide Component Map

The framework is structured as a layered system where high-level visual intent is translated into low-level programmatic execution.

| Harness Responsibility | Riptide Component | Implementation |
| :--- | :--- | :--- |
| **Observation** | Screenshot capture + AXTree injection | `chromedp.CaptureScreenshot`, `handleGetAccessibilityTree` |
| **Context** | Screenshot pruning, history management | `pruneOldScreenshots` in `pkg/computer/computer.go` |
| **Control** | Observe-Reason-Act loop | `computer.Run` in `pkg/computer/computer.go` |
| **Action** | CDP command dispatch | `pkg/computer/executor.go`, `tools_standard.go` |
| **State** | Browser context, session identity | `chromedp` allocator context |
| **Verification** | Post-action screenshot, DOM log, tool guard | `capturePostActionScreenshot`, `IsToolKnown` |

## Technical Deep Dive

### 1. The Action Executor (Physical Layer)
The Executor is the bridge between the model's visual interpretation and the browser's DOM.
*   **Standard Actions:** The agent supports the core Gemini Computer Use action set:
    *   `mouse_move`, `left_click`, `right_click`, `middle_click`, `double_click`.
    *   `left_click_drag` (Drag and Drop).
    *   `key` (Special keys/combinations), `type` (Text entry).
    *   `screenshot` (State capture), `cursor_position` (Coordinate verification).
*   **Coordinate Denormalization:** Translates the model's 0-1000 coordinate system into the exact pixel dimensions of the current viewport.
*   **Interaction Robustness:**
    *   **Spatial Aim Assist:** When a physical click is dispatched, the executor identifies the element at the coordinates. If the target is non-interactive, it performs a proximity search (Euclidean distance) to snap the interaction to the nearest focusable element.
    *   **Shadow DOM Penetration (Deep Hit):** Standard DOM queries often fail on modern web components. The executor uses recursive traversal to penetrate Shadow Roots, ensuring sliders and custom buttons are reachable.
    *   **Centered Focus:** To prevent layout "jumps" that confuse the model's visual context, the executor forces targeted elements into view using `block: 'center'` alignment.

### 2. Orchestration & Loop Management
The Orchestration layer manages the lifecycle of a session.
*   **State Observation:** Captures visual state (Screenshots) and structural state (DOM/Accessibility Tree).
    *   **Accessibility Tree (AXTree) Injection:** A simplified, textual representation of the browser's accessibility tree is injected alongside every screenshot. This allows the model to "read" the structure and values of interactive elements (like slider percentages or button labels) directly, complementing visual analysis and reducing errors from visual ambiguity.
*   **Context Pruning:** To maintain efficiency within the model's token limit, the loop manages a sliding window of recent screenshots, removing older visual data while retaining the text-based reasoning history.
*   **Safety Interception:** Intercepts `safety_decision` markers from the model, enabling automated acknowledgement or human-in-the-loop verification for sensitive actions.
*   **Prompt Injection Auto-Detection:** Gemini 3.5 Flash's built-in adversarial training (`EnablePromptInjectionDetection`) detects when page content attempts to hijack the agent's instructions. When triggered, the model stops with a safety `FinishReason` and Riptide terminates the session automatically, emitting an `EventPromptInjection` event.

### 3. Augmented Skills (The Hybrid Schema)
While the agent can navigate visually using the native `genai.ComputerUse` tool, certain tasks are better handled programmatically. Riptide classifies its tools into four categories to optimize both model performance and developer observability. We inject these custom capabilities alongside the native tools using a **Hybrid Tool Schema**.

*   **Standard (Native):** Core interactions defined by the Vertex AI Computer Use spec (e.g., `left_click`, `type`, `scroll`). Riptide acts purely as an executor for these.
*   **Skills (Custom Schemas):** Riptide-specific capabilities that provide high-reliability data (e.g., `get_page_layout` for DOM scanning, `get_accessibility_tree`). These are dynamically registered as `genai.FunctionDeclaration` objects.
*   **Patches (Alias Mappers):** Interceptors that catch common model hallucinations (e.g., mapping a hallucinated `scroll_down` call to `scroll(direction="down")`) to ensure the Vertex API doesn't crash with a 400 Invalid Argument error.
*   **Utility:** Framework-level helpers (e.g., `wait`, `navigate`) that manage the browser lifecycle.

For a complete breakdown of every tool and its category, see the [Tools Reference](tools_reference.md).

### 4. Context, Identity, & State Management
This layer defines the environment in which the agent operates.
*   **Session Persistence:** By mounting a persistent user data directory, the agent retains authentication state across runs.
*   **Identity Injection:** Support for injecting auth tokens or cookies directly into the browser context, allowing the agent to bypass login flows and start directly on the target task.

### 5. Identity & Transparency (The User Agent)

Riptide manages the browser's identity through the **User-Agent** string. This is the primary signal websites use to distinguish between human users and automated scripts.

*   **Polite Identification (Transparent Mode):** By default, Riptide appends an identifier to the User-Agent: `(Riptide; +https://github.com/ghchinoy/riptide)`. This follows the "Polite Crawler" convention, allowing site owners to identify the source of traffic and contact developers if needed.
*   **Realistic Defaults:** The base User-Agent is a modern, human-like Chrome on macOS string. This ensures high compatibility with sites that might block older or generic "headless" browser strings.
*   **Customization vs. Camouflage:**
    *   **Custom UA:** Developers can specify a completely custom string (e.g., to mimic a mobile device or a specific enterprise browser).
    *   **Anonymization:** By disabling `transparent-ua`, the Riptide identifier is removed, leaving only the realistic base string. This is useful for testing sites with overly restrictive or brittle bot detection.

## Observability & Debugging

The framework provides multiple layers of observability:
*   **Terminal UI (TUI):** Real-time monitoring of thinking, actions, and status.
*   **Structured Logs:** Detailed logs per session in `logs/session_<id>.log`.
*   **Visual Replays:** High-resolution screenshots and animated GIFs.
*   **Session Viewer:** A dedicated Lit-based web application for browsing session history and analyzing agent performance turn-by-turn.

## Deployment Scalability
The decoupled nature of the **Orchestration** and **Executor** allows for various deployment models:
*   **Standalone:** CLI-based execution where the loop and browser run on the same host.
*   **Distributed:** A central Orchestrator dispatching tasks to a fleet of headless browser containers running in a managed environment (e.g., Cloud Run or Kubernetes).
