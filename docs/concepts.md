# Website Assistant: Architectural Concepts

This document outlines the conceptual architecture of the **Website Assistant**. It maps standard software engineering terms to the specific components of our "Browser Operating System," providing a mental model for developers extending the framework.

## The "Browser OS" Analogy

Think of the Website Assistant not just as a script, but as an operating system that bridges Generative AI and the Web.

| Concept | The Analogy | Our Implementation | Responsibility |
| :--- | :--- | :--- | :--- |
| **The Driver** | Hardware Driver | `pkg/computer/executor.go` | **The Hands.** Translates abstract intent (e.g., "Click the blue button") into physical protocol commands (`chromedp.MouseClickXY`). Handles the raw I/O with the browser. |
| **The Kernel** | OS Kernel | `pkg/computer/computer.go` | **The Brain.** Manages the "Observe-Reason-Act" loop. It holds the context window, manages memory (history), and orchestrates the scheduling of tools. |
| **Skills** | Applications | Custom Go Functions | **The Actions.** Deterministic capabilities provided to the AI via Function Calling (e.g., `SaveToDatabase`, `ExtractTable`, `SolveCaptcha`). Used when visual interaction is insufficient. |
| **Modules** | Software Suites | Go Packages (e.g., `pkg/skills/shopify`) | **The Toolkit.** A logical grouping of Skills designed for a specific domain or web property (e.g., a "Shopify Module" containing navigation macros and API helpers). |
| **The Avatar** | User Profile | `chromedp` Context + Storage | **The Identity.** The persistent state of the user: Cookies, LocalStorage, Session Tokens, and User-Agent strings. Defines *who* is browsing. |

## Deep Dive: Components

### 1. The Driver (Physical Layer)
The Driver is the only component that touches the `chromedp` instance directly for *interaction*.
*   **Input:** Normalized coordinates (0-1000) and action types (`click`, `type`, `scroll`).
*   **Output:** Browser events.
*   **Driver Augmentation:** To compensate for AI inaccuracy, the Driver employs heuristics:
    *   **Aim Assist:** If a click misses a target, the driver searches the nearby DOM (spatial search) to "snap" to the nearest interactive element.
    *   **Auto-Focus:** Explicitly forces JavaScript focus on elements to ensure keystrokes aren't lost.

### 2. The Kernel (Orchestration Layer)
The Kernel manages the lifecycle of a task.
*   **Observe:** Captures the state (Screenshot + DOM).
*   **Reason:** Sends state to Gemini 2.5 (Vertex AI).
*   **Act:** Dispatches the model's response to the **Driver** (if it's a browser action) or a **Skill** (if it's a logical request).
*   **Safety:** Intercepts "Safety Decisions" (like CAPTCHAs) and negotiates with the model or prompts the human user.

### 3. Skills & Modules (Application Layer)
This is where the framework becomes extensible.
*   **Standard Computer Use:** The model uses the default mouse/keyboard skills.
*   **Hybrid Automation:**
    *   *Scenario:* Navigating a complex Single Page Application (SPA).
    *   *Technique:* **JavaScript Injection**.
    *   Instead of asking the AI to visually find a specific data attribute, a Skill can inject a JS snippet via `chromedp.Evaluate` to extract the data directly from the DOM.
    *   *Benefit:* 100% accuracy, zero hallucination, lower latency.

### 4. The Avatar (Persistence & Security)
The Avatar represents the "User Proxy."
*   **Persistence:** By mounting a persistent user data directory, the agent retains cookies and login sessions, avoiding the need to re-authenticate on every run.
*   **Secret Management:**
    *   *Anti-Pattern:* AI types a password character-by-character.
    *   *Secure Pattern:* The Go application injects the session cookie or auth token directly into the browser context from a secure vault (Env Var, Secret Manager). The AI simply wakes up "logged in."
*   **Human Handoff:**
    *   When the Avatar encounters a barrier it cannot solve (e.g., hardware 2FA key), it pauses execution and requests interaction from the real user via the CLI/UI.

## Future Direction: Distributed Fleet
This architecture supports scaling:
*   **Local:** One Avatar, One Kernel, One Driver (CLI).
*   **Cloud:** One API Gateway (Kernel) dispatching tasks to a fleet of containers (Driver + Avatar) running on Cloud Run, effectively offering "Computer Use as a Service."
