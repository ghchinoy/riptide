# Computer Use Safety Best Practices: An Honest Assessment

*Analysis supporting the Gemini 2.5 → 3.5 Flash transition blog post series.*

Google publishes safety best practices for the Computer Use API at `https://ai.google.dev/gemini-api/docs/computer-use#safety-best-practices`. This document audits how those practices are covered in the Google-provided Python reference implementations, how Riptide implements them, identifies real gaps, and — critically — examines which practices are well-grounded versus aspirational.

---

## The Five Published Best Practices

The documentation defines five recommendations and one hard requirement:

**Hard requirement (Terms of Service):**
> *"Customer agrees not to automatically bypass or circumvent any safety responses requiring end user human confirmation."*

**Recommendations:**

1. **Human-in-the-loop (HITL)** — When the model's internal safety system returns `safety_decision: require_confirmation`, the harness must present the decision to a human and obtain explicit approval before executing. Optionally, a custom system instruction can enumerate categories requiring confirmation before the model even reaches sensitive actions.

2. **Secure execution environment** — Run the agent in a sandboxed VM, container, or dedicated browser profile with limited permissions. Do not run against a primary browser profile containing personal credentials, cookies, or browsing history.

3. **Input sanitization** — Sanitize user-generated text in prompts to mitigate unintended instructions or prompt injection.

4. **Allowlists and blocklists** — Implement filtering mechanisms to control navigation targets. A domain blocklist is a floor; an allowlist is more restrictive.

5. **Observability and logging** — Log prompts, screenshots, model-suggested actions, safety responses, and all actions ultimately executed, for debugging, auditing, and incident response.

The documentation also describes the `safety_decision` field in detail and provides a reference implementation of the RULE 1 / RULE 2 system instruction pattern — a comprehensive enumeration of action categories requiring confirmation before execution.

---

## What the Python Reference Implementations Cover

Google provides two Python references relevant to Gemini 3.5 Flash: `agent.py` (the canonical computer-use-preview agent) and `intro_computer_use.py` (a Colab tutorial). The `playwright.py` browser backend in `computer-use-preview` is also relevant for environment security.

### `agent.py` (computer-use-preview)

| Practice | Coverage | Notes |
|---|---|---|
| HITL `safety_decision` | ✅ | `_get_safety_confirmation()` prompts user; sends `safety_acknowledgement: "true"` back |
| Custom system instruction (RULE 1/2) | ❌ | No system instruction in the config |
| `EnablePromptInjectionDetection` | ❌ | Not set; defaults to `false` |
| Secure environment (OS-level) | ❌ | Defers to the `Computer` backend |
| Input sanitization | ❌ | `query` passed directly |
| Allowlist/blocklist | ❌ | Not implemented |
| Logging/observability | ⚠️ | Rich table to console; no structured log file |
| Turn limit | ⚠️ | Exits on exception; no explicit `max_turns` |
| Screenshot pruning | ✅ | `MAX_RECENT_TURN_WITH_SCREENSHOTS = 3` |
| MALFORMED_FUNCTION_CALL retry | ✅ | Continues loop on malformed FC |
| Exponential backoff on API failure | ✅ | 5 retries with doubling delay |

**Assessment:** `agent.py` correctly implements `safety_decision` handling — the most important TOS-critical practice — and adds screenshot pruning. Everything else is absent. It is a correct minimal reference, not a comprehensive safe reference.

### `intro_computer_use.py` (Colab tutorial)

| Practice | Coverage | Notes |
|---|---|---|
| HITL `safety_decision` | ❌ | `execute_function_calls` has no `safety_decision` check |
| `EnablePromptInjectionDetection` | ❌ | Not set |
| Secure environment | ❌ | Bare Playwright launch, no hardening flags |
| Logging/observability | ❌ | `print()` statements only |
| Turn limit | ✅ | `max_turns=20` |
| Safety block handling | ✅ | Checks `not response.candidates` and breaks |

**Assessment:** The Colab tutorial is explicitly a learning artifact, not a production reference. It omits `safety_decision` handling entirely — a significant omission given its role as many developers' first exposure to the API.

### `playwright.py` (browser backend)

| Practice | Coverage | Notes |
|---|---|---|
| Chrome sandbox | ✅ | `--no-sandbox` deliberately omitted; sandbox stays ON |
| Browser hardening flags | ✅ | `--disable-extensions`, `--disable-file-system`, `--disable-plugins`, `--disable-background-networking`, `--disable-default-apps`, `--disable-sync` |
| Single-tab enforcement | ✅ | `on("page")` handler intercepts new tabs and redirects in current page |
| URL normalization | ✅ | Prepends `https://` if scheme missing |

**Assessment:** The strongest safety implementation in the reference suite. The hardening flags and Chrome sandbox decision are deliberate and production-appropriate.

---

## Riptide's Implementation

Riptide is a Go implementation of the agent harness. The following analysis is based on `pkg/computer/computer.go`, `pkg/computer/executor.go`, and `pkg/computer/events.go`.

| Practice | Status | Detail |
|---|---|---|
| HITL `safety_decision` | ✅ with critical gap | `EventSafety` + `SafetyHandler` contract + TUI prompt. **Gap:** when `SafetyHandler` is `nil` (headless/JSON mode), `approved` defaults to `true`, silently auto-approving all `require_confirmation` decisions. This is a TOS violation. Tracked as riptide-805.9 (P0). |
| `safety_acknowledgement` in FunctionResponse | ✅ | `executor.go` sets `safety_acknowledgement: true` before routing the action. |
| Custom system instruction | ⚠️ | System instruction names `safety_decision` and `prompt injection` explicitly but does not enumerate the RULE 1 action category list from the docs (ToS acceptance, CAPTCHA, financial transactions, communications, sensitive data, browser data, auth). |
| `EnablePromptInjectionDetection` | ✅ tested | Explicitly set `true` in `ComputerUse` config; unit-tested in `gemini35_test.go`. Ahead of both Python references. |
| Prompt injection response handling | ✅ | `isPromptInjectionResponse()` checks `FinishReasonSafety` and `FinishReasonProhibitedContent`; emits `EventPromptInjection`; terminates session cleanly. |
| Hallucination defense | ✅ | Unknown tool calls are intercepted before execution; history is corrected; `EventHallucination` is emitted. No 400 errors from Vertex AI. |
| Secure environment (Chrome sandbox) | ❌ inverted | `chromedp.NoSandbox` disables Chrome's internal process sandbox. `playwright.py` deliberately keeps the sandbox ON. The practical justification is that Chrome's sandbox requires kernel capabilities unavailable in standard Docker environments, but this is undocumented. |
| Browser hardening flags | ❌ | None of `playwright.py`'s `--disable-*` flags are applied to the chromedp allocator. |
| Input sanitization | ❌ | `--prompt` passed directly to model with no sanitization or validation. |
| Allowlist/blocklist | ❌ | URL is captured post-action for logging; no navigation filtering. |
| Observability and logging | ✅ strong | Structured `emit()` + `Observer` callback + session log file. Every event includes timestamp. Base64 image data truncated in logs. Deferred session-end event guarantees termination is always recorded. |
| Turn limit | ✅ | Hard `for i := 0; i < maxTurns` ceiling; configurable via `--max-turns`. |
| Per-call timeouts | ✅ | 90-second model API timeout; action-specific timeouts (navigate: 60s, page layout: 30s, key: 10s). |
| Context isolation | ✅ | Each operation gets a derived sub-context with independent timeout. Pre-flight context cancelled before agent loop starts. |
| Screenshot pruning | ✅ | Configurable `maxScreenshots`; tested in `gemini35_test.go`. |

### Where Riptide Leads the Reference Implementations

- **Prompt injection detection** — `EnablePromptInjectionDetection: true` with behavioral handling; neither Python reference sets this.
- **Structured observability** — Session log files, typed event system, `Observer` interface. The Python references use `print()`.
- **Hallucination defense** — History correction + alias mapper; the Python references raise `ValueError` on unknown tools.
- **Typed event system** — `SafetyHandler`, `Observer`, and the event type enum provide clean extension points; the Python references are monolithic.

### Where Riptide Falls Short

- **TOS violation (P0):** The `SafetyHandler` nil auto-approve path must be closed. See riptide-805.9.
- **System instruction completeness:** The RULE 1 category enumeration should be added. See riptide-69z.1.
- **Chrome sandbox:** The `NoSandbox` decision needs documentation and compensating controls. See riptide-69z.2.
- **Navigation filtering:** A domain blocklist is the practical floor. See riptide-69z.5.

---

## Are These Actually Best Practices? An Empirical View

The documentation frames five items as best practices. They are not equally well-grounded.

### Well-grounded: `safety_decision` HITL enforcement

The requirement to not auto-approve `require_confirmation` is correct and important. An agent that silently approves its own safety checks provides no meaningful human oversight. This is also the only item explicitly required by the Terms of Service.

**But there is an open empirical question:** does the model's internal safety system actually emit `safety_decision: require_confirmation` for the documented high-stakes categories? Cookie consent banners, mock payment buttons, CAPTCHA checkboxes, "Send Message" buttons — does the model reliably pause before each? If coverage is spotty, the RULE 1 system instruction pattern in the custom prompt is not optional: it becomes the primary safety layer. This is testable (riptide-69z.3) and the results will determine how much trust can actually be placed in the API-level mechanism.

### Well-grounded: prompt injection detection

`EnablePromptInjectionDetection` is a genuine safety mechanism. Prompt injection via malicious page content is a real attack vector for browser agents — a webpage can instruct the model to exfiltrate data, navigate to a different destination, or take actions the user never authorized.

**The empirical question:** what does it actually catch? Visible injected text? Hidden CSS text? Alt-attribute injection? Form label injection? The detection is based on the model's adversarial training, not a rule-based filter, so its coverage is not fully specified. Testing against a corpus of known injection patterns (riptide-69z.4) will map the actual boundary between what is and isn't caught.

### Real tension: secure execution environment vs. `NoSandbox`

The docs say: run in a sandboxed environment. `playwright.py` says: keep Chrome's internal sandbox ON. Riptide uses `chromedp.NoSandbox`.

These are two different layers. Chrome's internal process sandbox isolates renderer processes from the OS. OS-level sandboxing (Docker with seccomp profiles, VMs) isolates the entire agent from the host system. The two are complementary, not alternatives.

`NoSandbox` is a practical necessity in many headless Linux environments where Chrome's sandbox requires kernel capabilities (`CAP_SYS_ADMIN`) not available in standard containers. The right answer is: document this explicitly and recommend OS-level isolation as the compensating control. Running Riptide on a developer's primary machine without containerization means Chrome has full access to the developer's filesystem, cookies, and browsing history — which is exactly what the docs warn against.

**The claim that needs validation:** does OS-level isolation (Docker) with `NoSandbox` provide equivalent or better security than Chrome sandbox with no OS isolation? Almost certainly yes — but this should be stated, not assumed.

### Likely security theater: input sanitization of user prompts

The docs recommend sanitizing user-generated text in prompts. For Riptide, the user's `--prompt` flag is a trusted channel controlled by the developer running the tool. Sanitizing it protects against the user accidentally writing a prompt that conflicts with the system instruction — a narrow and unlikely failure mode.

The risk the docs likely have in mind is a *multi-tenant service* where untrusted end-users supply prompts. In that deployment model, sanitization matters. For a developer-facing CLI tool, the attack surface is negligible.

More importantly, sanitizing injection-pattern keywords from the prompt would break legitimate use cases. A prompt like `"Navigate to the page that says 'Ignore Previous Instructions' and take a screenshot"` is valid. Blocking it would be incorrect.

**The right answer:** input sanitization of user prompts is warranted for multi-tenant deployments where prompts come from untrusted sources. It is unnecessary for the developer-CLI use case Riptide currently targets. The assessment task (riptide-69z.6) should document this conclusion and flag the deployment context as the deciding variable.

### Partially security theater: allowlists and blocklists

A domain blocklist of known malicious sites is reasonable and implementable (riptide-69z.5). The claim that allowlists are "more secure" is correct in narrow deployments (a dedicated automation agent for a single service), but impractical for a general-purpose browser agent where legitimate task completion requires visiting arbitrary domains.

More fundamentally, an allowlist only prevents the *model* from navigating outside permitted domains. It does not prevent a malicious site *within* the allowlist from redirecting the agent, injecting content, or exfiltrating data via legitimate API calls. Allowlists reduce the attack surface without eliminating it.

---

## Coverage Matrix Summary

| Practice | Python `agent.py` | Python `playwright.py` | `intro_computer_use.py` | Riptide |
|---|---|---|---|---|
| HITL `safety_decision` (TOS) | ✅ | — | ❌ | ✅ / ⚠️ nil gap |
| Custom RULE 1 system instruction | ❌ | — | ❌ | ⚠️ partial |
| `safety_acknowledgement` in response | ✅ | — | ❌ | ✅ |
| `EnablePromptInjectionDetection` | ❌ | — | ❌ | ✅ tested |
| Injection response handling | ❌ | — | ❌ | ✅ |
| Hallucination defense | ❌ raises | — | ❌ | ✅ |
| Chrome sandbox ON | — | ✅ | ❌ | ❌ NoSandbox |
| Browser hardening flags | — | ✅ | ❌ | ❌ |
| Single-tab enforcement | — | ✅ | — | not observed |
| Input sanitization | ❌ | — | ❌ | ❌ |
| Domain blocklist | ❌ | ❌ | ❌ | ❌ planned |
| Structured logging/observability | ❌ | — | ❌ | ✅ |
| Turn limits | ⚠️ | — | ✅ | ✅ |
| Screenshot pruning | ✅ | — | ❌ | ✅ |

---

## Open Empirical Questions

The following questions require running experiments against real model behavior. Results will be documented in `docs/validation/` and inform the blog post series:

1. **`safety_decision` coverage** — Which action categories reliably trigger `require_confirmation`? Which are missed? How does the RULE 1 system instruction change coverage? *(riptide-69z.3)*

2. **Prompt injection detection coverage** — What injection patterns does `EnablePromptInjectionDetection` catch? Visible injected text, hidden text, alt-attribute injection, form label injection, role confusion? *(riptide-69z.4)*

3. **System instruction vs. API detection** — For categories where the API-level detection misses, does the RULE 1 behavioral system instruction in the prompt compensate? Are both layers required?

4. **NoSandbox risk profile** — In Riptide's typical deployment context (developer laptop, containerized CI), what is the practical risk of `NoSandbox`? What kernel capabilities does Chrome's sandbox actually need in a Linux container?

5. **Allowlist security ceiling** — Given that malicious sites within an allowlist can still exfiltrate data via legitimate redirects or API calls, what actual security guarantee does a domain allowlist provide?

---

## Implications for the 2.5 → 3.5 Transition

Gemini 3.5 Flash introduces `EnablePromptInjectionDetection` as a new capability — this was not available in 2.5 Preview. The transition is therefore not just a performance upgrade; it adds a model-level safety mechanism that the harness can now lean on.

Key messages for the blog series:

- **Riptide enables the new safety capability by default.** The Python references do not set `EnablePromptInjectionDetection`, leaving developers to discover it independently. Riptide surfaces it as a first-class, tested feature.
- **`safety_decision` compliance is a TOS requirement, not a suggestion.** The nil auto-approve gap (riptide-805.9) is the most important safety fix in this cycle — not for performance reasons but for legal compliance reasons.
- **The reference implementation and the published best practices don't fully agree.** `agent.py` omits prompt injection detection. `intro_computer_use.py` omits `safety_decision` handling. Developers following the Colab tutorial are starting without a critical safety control in place.
- **Riptide's Go architecture provides structural advantages.** The typed `SafetyHandler` and `Observer` interfaces make safety controls first-class extension points, not bolted-on conditionals. The alias mapper and hallucination correction are absent from the Python references. These are concrete harness value adds beyond language choice.
