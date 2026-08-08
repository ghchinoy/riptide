# Blog Post Series: Building Production-Grade Computer Use Agents with Gemini 3.5 Flash

A six-part series covering Riptide's development, architecture, and evaluation — intended for the Gemini developer community and the broader agent engineering audience.

## Series Arc

The series moves from **technical migration** → **architectural argument** → **safety** → **deployment** → **benchmarking** → **use case extension**. Each post stands alone but references earlier ones where relevant.

## Status & Publication Dependencies

```
Post 1 (Migration)     ── shipped ──────────────────── published to website
Post 4 (Sandboxing)    ── shipped ──────────────────── published to website
Post 3 (Safety)        ── needs riptide-69z results ── draft after 69z.3, 69z.4
Post 5 (Benchmarking)  ── needs riptide-ipo results ── draft after ipo.2, ipo.3
Post 6 (BDD Testing)   ── needs riptide-pi8 impl ───── draft after pi8.1, mkq.5
Post 2 (The Harness)   ── DEFERRED to dev note ─────── see below
```

### Post 2 deferred (2026-06)

Post 2 is held as a **development note**, not a publishable draft. The "why harness
design matters" argument needs comparative data we don't have yet — on the simple
sites tested so far, a robust harness and the flat reference perform about the same.
The advantages should surface on complex sites, the benchmark suite, or desktop
usage.

**Publish trigger** — any one of:
1. `riptide-0oo.6` — tiered vs flat harness measurement on complexity-graded sites
2. `riptide-ipo` — benchmark scores showing harness contribution
3. Desktop computer-use tasks (future)

What survives into the eventual post regardless: the reliability findings
(`EOFError` / `BlockReason` crashes the reference, Riptide handles both) and the
hyt profiling story (per-turn overhead). Neither alone justifies the post.

Recommended order for what ships next: **3 → 5 → 6**, with **2** slotting in once
its trigger data lands (likely alongside or after 5).

## Source Material Map

| Source Document | Feeds Posts |
|---|---|
| `docs/gemini-25-to-35-migration.md` | Post 1 (primary), Post 2 (supporting) |
| `docs/lessons_learned.md` | Post 1 (supporting), Post 2 (supporting) |
| `docs/concepts.md` | Post 2 (primary) |
| `docs/safety_best_practices_analysis.md` | Post 3 (primary) |
| `docs/runtime_isolation.md` | Post 4 (primary) |
| `docs/benchmarks.md` | Post 5 (primary) |
| `docs/validation/` results | Posts 2, 3, 5 |
| `pkg/computer/skills/ux_tester_prompt.md` | Post 6 (primary) |

## Posts

- [Post 1: Computer Use Grows Up](post-01-migration.md) — *shipped*
- [Post 4: Sandboxing Browser Agents](post-04-sandboxing.md) — *shipped*
- [Post 3: Trust But Verify — Safety Best Practices, Tested](post-03-safety.md) — *pending 69z*
- [Post 5: How Does Your Computer Use Agent Actually Perform?](post-05-benchmarking.md) — *pending ipo*
- [Post 6: Riptide as a Testing Agent](post-06-bdd-testing.md) — *pending pi8*
- [Dev note: What a Robust Harness Adds](post-02-the-harness.md) — *deferred, was Post 2*
