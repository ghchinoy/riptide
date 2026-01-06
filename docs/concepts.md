# Riptide: Architectural Concepts

This document outlines the technical architecture of **Riptide**. It defines the core components and their responsibilities in bridging Generative AI with browser-based automation.

## Core Component Architecture

The framework is structured as a layered system where high-level visual intent is translated into low-level programmatic execution.

| Component | Implementation | Responsibility |
| :--- | :--- | :--- |
| **Action Executor** | `pkg/computer/executor.go` | **The Physical Layer.** Translates normalized coordinates (0-1000) and abstract actions into Chrome DevTools Protocol (CDP) commands. |
| **Orchestration Loop** | `pkg/computer/computer.go` | **The Control Plane.** Manages the "Observe-Reason-Act" cycle, maintains conversation history, and handles model communication. |
| **Augmented Skills** | Custom Go Functions | **Deterministic Extensions.** Provides high-reliability capabilities (e.g., direct DOM extraction) via GenAI Function Calling. |
| **State & Identity** | `chromedp` Context + Storage | **Persistence Management.** Manages cookies, local storage, and session state to maintain identity across interactions. |

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
    *   **Accessibility Tree (AXTree) Injection:** A future enhancement where a simplified, textual representation of the browser's accessibility tree is provided to the model. This allows the model to "read" the structure and values of interactive elements (like slider percentages or button labels) directly, complementing visual analysis and reducing errors from visual ambiguity.
*   **Context Pruning:** To maintain efficiency within the model's token limit, the loop manages a sliding window of recent screenshots, removing older visual data while retaining the text-based reasoning history.
*   **Safety Interception:** Intercepts `safety_decision` markers from the model, enabling automated acknowledgement or human-in-the-loop verification for sensitive actions.

### 3. Augmented Skills (Application Layer)
While the agent can navigate visually, certain tasks are better handled programmatically.
*   **Hybrid Automation:** Developers can inject "Skills" that use `chromedp.Evaluate` to perform complex extractions or interactions that are visually ambiguous but programmatically trivial.
*   **Tactile Feedback:** Future iterations will return programmatic values (e.g., current slider position or input text) back to the model in the function response, providing immediate verification of action success.

### 4. Context, Identity, & State Management
This layer defines the environment in which the agent operates.
*   **Session Persistence:** By mounting a persistent user data directory, the agent retains authentication state across runs.
*   **Identity Injection:** Support for injecting auth tokens or cookies directly into the browser context, allowing the agent to bypass login flows and start directly on the target task.

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
