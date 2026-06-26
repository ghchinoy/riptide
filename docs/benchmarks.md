# Computer Use Agent Benchmarks

A survey of established benchmarks for evaluating web and GUI agents, with assessment of each benchmark's fit for Riptide's architecture and validation goals.

## Landscape Overview

Benchmarks in this space split along two axes: **environment** (offline replay vs. live browser vs. VM-hosted) and **scope** (minimal widgets vs. realistic web vs. full desktop OS). Riptide is a live-browser web agent, so benchmarks requiring VMs or desktop OS environments are poor fits for core validation, though they may be relevant for broad capability comparisons.

| Benchmark | Year | Environment | Scope | Network | Riptide Fit |
|---|---|---|---|---|---|
| MiniWob++ | 2017/2018 | Static HTML | Web widgets | None | High |
| Mind2Web | 2023 | DOM snapshots | Real websites (offline) | None | Medium |
| WebArena | 2023 | Self-hosted | Realistic web apps | Self-hosted | High |
| AgentBench | 2023 | Multi-env | Web + OS + code | Mixed | Medium |
| AssistGUI | 2023 | Screenshots | Windows desktop | None | Low |
| VisualWebArena | 2024 | Self-hosted | Visual web tasks | Self-hosted | High |
| OSWorld | 2024 | VM (Win/Ubuntu) | Web + desktop OS | Real + VM | Low (initially) |
| WorkArena | 2024 | SaaS | ServiceNow enterprise | Live SaaS | Low |
| AndroidWorld | 2024 | Android emulator | Mobile apps | Emulator | None |
| WindowsAgentArena | 2024 | VM (Windows) | Windows desktop | VM | Low |
| Spider2-V | 2024 | Mixed | Data science/coding | Mixed | Low |

---

## Benchmarks in Detail

### MiniWob++ — Offline, CI-safe

**Source:** Shi et al. (2017), extended by Liu et al. (2018)  
**Tasks:** ~100 task types covering atomic web interactions — clicking buttons by label, entering text, selecting from dropdowns, navigating calendars, date pickers, checkboxes, drag-and-drop.  
**Environment:** Self-contained static HTML pages served locally. No external dependencies.  
**Evaluation:** Deterministic pass/fail based on form state or DOM condition after agent action.

**Why it fits Riptide:**
- Structurally identical to `cmd/testserver` — Riptide already has offline test infrastructure in this shape
- No CAPTCHA, no auth, no network variance
- Fast: each task completes in 1–5 turns
- CI-safe: can run in the same `go test` pass as executor unit tests
- Good coverage of the interaction primitives Riptide's executor implements

**Limitations:** Tasks are minimal and stylized. Strong performance here does not predict performance on realistic multi-step web tasks.

**Adoption path:** Low friction. Extend `cmd/testserver` with MiniWob++ HTML fixtures, or run directly against the MiniWob++ server. Map task IDs to Riptide prompts.

---

### WebArena — Self-hosted, Realistic

**Source:** Zhou et al. (2023)  
**Tasks:** 812 tasks across five realistic web applications, requiring multi-step reasoning and cross-site coordination.  
**Applications (self-hosted):**
- **Postmill** — Reddit-like forum
- **GitLab** — full GitLab CE instance
- **OneStopShop** — e-commerce (Magento)
- **Wikipedia** — local mirror
- **phpBB** — forum/CMS

**Environment:** Docker Compose stack, no real network required once initialized. Tasks are sandboxed.  
**Evaluation:** Rule-based (URL match, DOM state, API call) and LLM-judge hybrid.

**Why it fits Riptide:**
- No CAPTCHA, no real-network variance — reproducible results
- Tasks are grounded in actual web app interactions (login, post creation, issue filing, search)
- Directly exercises Riptide's core loop: multi-turn, auth-aware, form-filling, navigation
- Gold standard in the research literature — results are comparable across papers
- Self-hosted removes the dependency on riptide-dc9 (CAPTCHA detection)

**Limitations:** Docker stack setup is non-trivial. Tasks require seeded application state (accounts, posts) to be consistent across runs.

**Adoption path:** Medium friction. Run Docker Compose stack, configure Riptide with site credentials via session persistence, map WebArena task JSONs to `riptide run --prompt`.

---

### VisualWebArena — Self-hosted, Visual Grounding

**Source:** Koh et al. (2024)  
**Tasks:** Extends WebArena with tasks that require identifying UI elements by visual appearance — finding a specific image, matching a color, locating a logo — rather than by text label alone.  
**Environment:** Same Docker stack as WebArena with additional image-rich task fixtures.  
**Evaluation:** LLM-judge + visual similarity metrics.

**Why it fits Riptide:**
- Directly exercises the screenshot-first observation model that Riptide uses with Gemini 3.5 Flash
- Complementary to WebArena: isolates visual grounding capability specifically
- AXTree injection (Riptide's `--axt` flag) is particularly relevant here — tasks where AXT helps vs. hurts are identifiable

**Limitations:** Requires the same Docker setup as WebArena. LLM-judge evaluation adds cost.

**Adoption path:** Low marginal friction if WebArena is already running — VisualWebArena shares the same infrastructure.

---

### Mind2Web — Offline Replay

**Source:** Deng et al. (2023)  
**Tasks:** 2,350 tasks across 137 real websites, derived from human demonstrations. Includes DOM snapshots, action sequences, and annotations.  
**Environment:** Offline evaluation against recorded DOM states — no live browser required.  
**Evaluation:** Element match accuracy against gold action sequences.

**Why it partially fits Riptide:**
- Large task diversity across many real websites
- Useful for evaluating action selection logic in isolation (without running a full browser session)
- No infrastructure cost

**Limitations:** Offline replay diverges from live execution. Riptide's executor makes live CDP calls; evaluating against frozen DOM snapshots misses network, timing, and dynamic content behavior. The benchmark is more useful for fine-tuning or element selection models than for harness validation.

**Adoption path:** Medium friction, but limited value for Riptide's specific architecture. Better suited for offline element-prediction ablations.

---

### OSWorld — VM-hosted, Broad Coverage

**Source:** Xie et al. (2024)  
**Tasks:** 369 tasks across Windows, Ubuntu, and macOS VMs, covering web browsers (Chrome), productivity software (LibreOffice, VS Code), and OS-level interactions.  
**Environment:** VM-based (VMware/VirtualBox), Python eval harness, requires significant compute and setup.  
**Evaluation:** Programmatic state-based (file content, app state, DOM checks).

**Why it's cited but not ideal:**
- Cited in the Gemini 3.5 Flash blog post — provides an external reference point for reported model scores
- Broad scope includes web tasks, but the web subset is a minority of total tasks
- VM infra is heavy: requires a separate Python eval harness, VM images, and orchestration outside of Riptide's Go toolchain

**Limitations:** The mismatch between OSWorld's VM eval harness and Riptide's live-browser Go architecture makes direct integration awkward. "OSWorld-aligned" in practice means manually porting task descriptions and running them as `riptide run --prompt` without the OSWorld eval harness — losing the programmatic evaluation layer.

**Adoption path:** High friction. Recommended as a deferred, manually-run comparison rather than an integrated benchmark. Do not block riptide-805 progress on OSWorld.

---

### AgentBench — Multi-environment

**Source:** Liu et al. (2023)  
**Tasks:** Eight environments including web shopping, web browsing, OS shell, database, knowledge graph, lateral thinking puzzles, and coding.  
**Environment:** Mixed — some Docker-hosted, some sandboxed.

**Why it partially fits Riptide:**
- Web shopping and web browsing environments are relevant
- Provides a multi-domain view of agent capability

**Limitations:** Riptide is a browser-only agent; non-browser environments in AgentBench are irrelevant. The web subsets are less developed than WebArena.

---

### Low-Fit Benchmarks (for completeness)

| Benchmark | Reason for low fit |
|---|---|
| **AssistGUI** | Windows desktop GUI, screenshot-only, no live execution |
| **WorkArena** | ServiceNow-specific, requires paid/trial SaaS instance |
| **AndroidWorld** | Android emulator, mobile apps — not web browser |
| **WindowsAgentArena** | Windows VM, desktop apps — not web browser |
| **Spider2-V** | Data science / coding tasks — different domain entirely |

---

## Recommended Adoption Path for Riptide

### Phase 1: CI-safe offline (low friction, start here)
Use **MiniWob++** task fixtures. Extend `cmd/testserver` with MiniWob++ HTML pages, or run against the MiniWob++ static server. Map ~20 representative task types to Riptide prompts and add them to `tests/benchmark/`. These run in CI without network or credentials.

### Phase 2: Realistic sandboxed web (medium friction, next)
Stand up **WebArena** (+ **VisualWebArena**) via Docker Compose. This is the highest-value benchmark for Riptide's core loop. Run as a scheduled job or on-demand, not in CI. Results are reproducible and directly comparable to published agent scores.

### Phase 3: External comparison point (high friction, deferred)
Run a manual, non-integrated comparison against **OSWorld** web-browser tasks to generate a reference score. Do not attempt to integrate the OSWorld Python eval harness into Riptide's Go toolchain — the architectural mismatch makes this a poor use of time. Run OSWorld tasks as `riptide run --prompt` with manual pass/fail assessment against OSWorld's published task descriptions.

---

## Relationship to Riptide Validation

Benchmarks are distinct from Riptide's pragmatic validation suite:

| Concern | Goal | Where |
|---|---|---|
| **Smoke tests** | Does the harness start and complete a basic task? | `riptide-805` validation epic |
| **Regression tests** | Did a code change break hallucination recovery, injection detection, or pruning? | `riptide-805` validation epic |
| **Harness comparison** | Does the harness add measurable value over bare model? | `riptide-805` validation epic |
| **Benchmark scoring** | How does Riptide+Gemini 3.5 Flash score on standard tasks? | Separate benchmark epic |

Validation gates ship quality. Benchmarks characterize capability. Both are useful; mixing them in one epic conflates release confidence with research metrics.
