# Riptide Documentation Index

## Architecture & Design

| Document | Description |
|---|---|
| [concepts.md](concepts.md) | Agent harness concepts, Riptide component map, technical deep-dive |
| [tools_reference.md](tools_reference.md) | Complete tool registry reference: Standard, Skill, Patch, Utility categories |
| [runtime_isolation.md](runtime_isolation.md) | Runtime isolation analysis: Apple Container, Docker, Cloud Run Jobs, GKE Agent Sandbox |
| [thoughts.md](thoughts.md) | Early design notes on local, visible-browser, and distributed deployment modes |

## Engineering Reference

| Document | Description |
|---|---|
| [gemini-25-to-35-migration.md](gemini-25-to-35-migration.md) | Concrete migration guide: API changes, function call name mapping, config additions |
| [lessons_learned.md](lessons_learned.md) | Coordinate drift, hallucinated tools, viewport stability — what broke and how we fixed it |
| [safety_best_practices_analysis.md](safety_best_practices_analysis.md) | Audit of Google's published safety best practices vs Python reference implementations vs Riptide |
| [benchmarks.md](benchmarks.md) | Computer use benchmark landscape: MiniWob++, WebArena, OSWorld, and fit assessment for Riptide |

## Validation

| Document | Description |
|---|---|
| [test_scenarios.md](test_scenarios.md) | Testserver scenario definitions: infinite scroll, Shadow DOM, CAPTCHA barrier |
| [validation/tier1_scenarios.md](validation/tier1_scenarios.md) | Tier 1 smoke test scenario specifications |
| [validation/tier2_configs.md](validation/tier2_configs.md) | Tier 2 harness-vs-bare comparison configuration |

## Visual Assets

| Document | Description |
|---|---|
| [image-prompts.md](image-prompts.md) | NanoBanana prompts for blog post headers and illustrations; style notes and templates for future posts |

Graphviz source files (`.dot`) live alongside their rendered `.webp` outputs. Regenerate with `dot -Tpng file.dot -o file.png && cwebp file.png -o file.webp -q 90`.

## Blog Post Series

See [blog/README.md](blog/README.md) for the full series overview and status.

| Post | Title | Status |
|---|---|---|
| [01](blog/post-01-migration.md) | Computer Use Grows Up: Migrating to Gemini 3.5 Flash | Ready to draft |
| [02](blog/post-02-the-harness.md) | Go vs Python: Building a Better Computer Use Harness | Awaiting 805 results |
| [03](blog/post-03-safety.md) | Trust But Verify: Safety Best Practices, Tested | Outline ready; awaiting 69z results |
| [04](blog/post-04-sandboxing.md) | Sandboxing Browser Agents | Ready to draft |
| [05](blog/post-05-benchmarking.md) | How Does Your Computer Use Agent Actually Perform? | Future (awaiting riptide-ipo) |
| [06](blog/post-06-bdd-testing.md) | Riptide as a Testing Agent: Gherkin + Computer Use | Future (awaiting riptide-pi8) |
