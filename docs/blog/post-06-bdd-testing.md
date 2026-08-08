# Post 6: Riptide as a Testing Agent — Gherkin + Computer Use

**Status:** Conceptual outline only — awaiting riptide-pi8 implementation  
**Source material:** `pkg/computer/skills/ux_tester_prompt.md`, riptide-pi8 deliverables  
**Target audience:** QA engineers, frontend developers, teams using BDD workflows  
**Estimated length:** 1,800–2,400 words  
**Publication dependency:** riptide-pi8.1 (Scout mode), riptide-mkq.5 (BDD execution), riptide-mkq.6 (step verification)  
**bd epic:** riptide-pi8

---

## Angle

Most computer use agent demos show an agent doing something useful on the web. This post shows an agent doing something useful *for developers*: writing tests. Riptide's Scout mode takes a URL, visually inventories the page, and outputs a draft Gherkin `.feature` file. The BDD execution mode takes that file and runs it — the model interprets each Given/When/Then step as a computer use task. The Then verification step produces structured pass/fail output.

This is a different kind of agent use case: not "do this task for me" but "tell me what tasks are possible and test that they work."

---

## Outline

### Hook / Intro (150 words)
- Writing automated UI tests is slow and brittle. Writing them for pages you don't control yet is slower.
- Scout mode: point Riptide at a URL, get back a `.feature` file. No test framework setup, no CSS selectors, no waiting for designers to finish.
- The thesis: computer use models are already good at visual reasoning about UI structure. BDD test authoring is just that reasoning, formalized.

### 1. The UX Tester Skill (200 words)
- `pkg/computer/skills/ux_tester_prompt.md`: the system prompt that makes Riptide behave as a Junior UX Tester
- What it constrains: structured observation, conservative action, explicit uncertainty
- How it's injected: `buildSystemInstruction()` with the skill loaded at session start
- The precedent this sets for other skill injections

### 2. Scout Mode: Page → Gherkin Draft (400 words)
*To be filled with riptide-pi8.1 implementation*
- `riptide run --mode scout --url https://example.com`
- What the agent does: navigate, take screenshot, use `get_page_layout` / `get_accessibility_tree` to enumerate interactive elements, reason about user journeys, output `.feature` file
- Example output: a login page becomes a `login.feature` with Given/When/Then for happy path and two error cases
- Quality of output: not a finished test suite, a good starting draft — how much editing does it typically need?
- The Prompt-to-Gherkin rewriter (riptide-pi8.4): `riptide run --prompt "check if login works"` → internally rewrites to Gherkin before execution

### 3. Case Study: Hybrid Human-Agent Gherkin Scenario ("Escape the Mines")
- **Scenario Specification:** `docs/test_scenarios/maze_game.feature`
- **Validation Log:** `docs/validation/maze_game_results.md`
- **The Setup:** A human developer launches Chrome locally (`--remote-debugging-port=9222`), navigates to `https://www.lotr.com/games/maze`, and leaves the tab open. Riptide attaches via `--attach http://127.0.0.1:9222 --tab-id <ID>`.
- **Rules Discovery Phase:** Before attempting full execution, Riptide runs short observation/probe prompts to empirically discover game mechanics:
  - Probe 1 (Clicks): Clicking the "Run" button does not start the game (`DIV`/`H2` element focus).
  - Probe 2 (Keyboard): `press_key(key="Enter")` immediately starts the game (`Progress 0%`, `Time 0s`).
  - Probe 3 (Movement): On-screen directional arrows (▲ ◄ ▼ ►) are non-interactive decorations; `press_key` with `ArrowUp`/`ArrowDown`/`ArrowLeft`/`ArrowRight` dispatches native CDP key events (`\u0304`, `\u0301`, `\u0302`, `\u0303`).
- **Tab Preservation:** Demonstrates Riptide's target-preservation logic — Riptide detaches cleanly without issuing `Target.CloseTarget`, leaving the user's browser tab open across consecutive runs.

### 4. Step Verification: Pass/Fail Reporting (300 words)
*To be filled with riptide-mkq.6 and riptide-pi8.2 implementation*
- The Then problem: "Then I should see a success message" requires visual + structural verification
- How Riptide verifies: screenshot analysis + DOM state check via `get_computed_style` / `get_page_layout`
- Structured output: JSON report mapping each step to PASS/FAIL/SKIP with screenshot evidence
- Integration with CI: `riptide run --mode bdd --json` outputs machine-readable results for test pipeline consumption

### 5. The Gherkin Training Corpus (200 words)
*From riptide-pi8.3 research*
- The system prompt (`ux_tester_prompt.md`) trains the model on Gherkin conventions specific to computer use
- What makes Gherkin effective for computer use vs traditional UI testing: natural language steps, no selectors, model handles ambiguity
- Where it breaks down: precise numerical assertions ("slider should be at 75%"), multi-tab behavior, file system interactions

### 6. Connection to the Benchmark Corpus (200 words)
- The custom benchmark corpus (Post 5) and the BDD corpus are the same artifact, viewed differently
- A Scout-generated `.feature` file for a stable, offline testserver page is a benchmark entry
- The graduation path: Scout writes a draft → engineer reviews and adds success predicates → entry joins the benchmark corpus → runs on every CI build
- This closes the loop: computer use agents writing tests for computer use agents

### Conclusion (150 words)
- BDD mode inverts the usual agent use case: instead of doing tasks, the agent describes and verifies them
- This is a natural fit for teams already using Gherkin — it lowers the barrier to authoring computer use test scenarios
- It's also a forcing function for harness quality: if the agent can't reliably verify a Then step, the verification layer needs work before the scenario can be a benchmark

---

## Key Deliverables to Document
*(to be filled when riptide-pi8 tasks complete)*

- Scout mode: example `.feature` file output for a real page
- BDD execution: step-by-step trace of a `riptide run --mode bdd` session
- Verification output: JSON pass/fail report format
- Prompt-to-Gherkin: example rewrite from free-text prompt to formal scenario

## Open Questions for Implementation
- What is the right confidence threshold for Then step verification?
- Should ambiguous steps ask the user (safety_decision) or attempt and report uncertainty?
- How does BDD mode interact with the session viewer — can steps be replayed step-by-step?
