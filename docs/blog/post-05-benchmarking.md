# Post 5: How Does Your Computer Use Agent Actually Perform?

**Status:** Early outline only — awaiting riptide-ipo work  
**Source material:** `docs/benchmarks.md`, future `docs/validation/` results  
**Target audience:** Researchers and developers comparing agent implementations  
**Estimated length:** 2,000–2,800 words  
**Publication dependency:** riptide-ipo.2 (MiniWob++), riptide-ipo.3 (WebArena), riptide-ipo.4 (token efficiency)  
**bd epic:** riptide-ipo

---

## Angle

"Computer use agent" is not a precise description. A model can complete 95% of a task and still fail it. It can complete a task in 4 turns or 40. It can use 2,000 tokens or 20,000. Benchmarking answers the questions that anecdotes can't: on a defined set of tasks, at a defined difficulty level, with defined success criteria, how does your implementation actually perform?

This post covers the benchmark landscape for computer use agents, the infrastructure choices we made for Riptide's evaluation, and our results on MiniWob++ and WebArena.

---

## Outline

### Hook / Intro (150 words)
- The Gemini 3.5 Flash blog post cites OSWorld scores. What does that actually tell you?
- OSWorld is a VM-based benchmark with a Python eval harness. Riptide is a Go harness running against a live Chromium instance. The architectures are different enough that "apples-to-apples" is a stretch.
- More useful: benchmarks designed to run against your actual deployment infrastructure, producing scores that measure *your harness + model* combination under conditions that resemble real use.

### 1. The Benchmark Landscape (400 words)
- Draw from `docs/benchmarks.md`: MiniWob++, WebArena, VisualWebArena, Mind2Web, OSWorld, AgentBench
- Categorize by: offline/self-hosted/live, network requirements, Chrome compatibility, score comparability
- The three-phase adoption path: MiniWob++ (offline, CI-safe) → WebArena (self-hosted, no CAPTCHA) → OSWorld (manual reference comparison)

### 2. Why Not OSWorld First (200 words)
- OSWorld is the benchmark Google cites. It's also the hardest to run.
- VM infrastructure, Python eval harness, 369 tasks across multiple operating systems — it's not designed to run against a Go + Chromium harness without significant adaptation
- The architectural mismatch: OSWorld's eval harness evaluates final state; Riptide's session log captures the trajectory
- OSWorld as a *reference point*, not an integrated benchmark — run a subset manually, compare to published scores, document honestly

### 3. Phase 1: MiniWob++ Results (500 words)
*To be filled with riptide-ipo.2 results*
- What MiniWob++ tests: atomic web interactions (click-button-by-label, select-from-dropdown, enter-text, calendar navigation)
- Why CI-safe matters: these run in the same `go test ./...` pipeline as unit tests; no network, no credentials, deterministic
- Our testserver extensions: which MiniWob++ task types mapped to existing testserver fixtures vs required new HTML
- Results table: task completion rate by category (clicks, forms, navigation, dynamic content)
- Token efficiency: input/output tokens per task, turns per completion
- Failure analysis: what kinds of tasks fail and why

### 4. Phase 2: WebArena Results (500 words)
*To be filled with riptide-ipo.3 results*
- What WebArena tests: realistic multi-step web tasks on self-hosted apps (Postmill/Reddit, GitLab, e-commerce, Wikipedia, phpBB)
- Infrastructure: Docker Compose stack; no real network required; reproducible across runs
- Task selection: 20 tasks spanning all five apps, with emphasis on tasks that exercise Riptide's harness features (AXTree injection, aim assist, multi-turn state)
- VisualWebArena subset: tasks requiring visual grounding — measures the value of `--axt` flag
- Results table: completion rate, average turns, token usage, by app and task type
- Comparison: `--axt` on vs off; with vs without Aim Assist; harness vs bare model (Tier 2 connection)

### 5. Token Efficiency Analysis (300 words)
*To be filled with riptide-ipo.4 results*
- Per-session token breakdown: input tokens (screenshots + history + system instruction), output tokens (tool calls + reasoning)
- The screenshot pruning impact: tokens saved by keeping only last N screenshots vs full history
- The AXTree overhead vs benefit: AXTree adds input tokens; does it reduce turns enough to be token-efficient?
- Comparison to Python reference (if available): same tasks, same model, different harness overhead

### 6. Toward a Custom Benchmark Corpus (300 words)
- The long-term trajectory: pragmatic validation tasks (riptide-805) → stabilize → formalize as benchmark
- What makes a good custom benchmark: deterministic success predicates, offline-capable, covers harness-specific behaviors (hallucination recovery, injection detection, context pruning)
- The `BenchmarkTask{Prompt, StartURL, SuccessPredicate, MaxTurns}` struct from riptide-ipo.2
- The graduation path: a task that was a smoke test becomes a benchmark entry when it has a machine-readable success predicate and seed-stable behavior

### Conclusion (150 words)
- Benchmarking is how "this works" becomes "this works, measured, reproducible, improving"
- MiniWob++ and WebArena scores are not the goal — they're a calibration mechanism
- The custom corpus is where Riptide's specific harness behaviors (hallucination defense, injection detection, AXTree grounding) get measured directly rather than inferred from general benchmark tasks

---

## Key Data Placeholders

- MiniWob++ completion rate by task category
- WebArena completion rate by application
- Token efficiency: tokens/task, turns/task, with/without AXTree
- Failure categorization: hallucination, injection, timing, element-not-found

## Infrastructure Notes for Writing
- Cloud Run Jobs batch dispatch (from riptide-3ya.8) is the production path for running N×M benchmark tasks — mention it as the mechanism, not the finding
- The session log parser (riptide-805.7) generates the data for all result tables — worth one paragraph on the data pipeline
