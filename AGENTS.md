# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

## Troubleshooting & Interaction

*   **Visual & Structural Awareness:** Use `get_page_layout` to obtain a text-based map of interactive elements if the screenshot is ambiguous or if elements are off-screen.
*   **Precision Verification:** Use `get_computed_style` to verify exact states (e.g., slider values, colors, visibility) that are hard to confirm via screenshot alone.
*   **Audit Mode:** When running with `--mode audit`, focus on structural and visual health violations (contrast, overflow).
*   **Full-Page Context:** If an interaction fails below the fold, check the `turn_N_full.png` screenshot in the session directory to see if the element was misaligned or moved.
*   **Viewport Stability:** Be aware that some sites auto-scroll or resize on interaction. If the agent is stuck in a scroll loop, suggest using specific `click_at` coordinates on a non-moving anchor first.
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

