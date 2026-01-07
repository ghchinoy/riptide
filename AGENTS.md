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

## Operational Persistence & Backend Debugging

When managing background processes (like the Session Viewer backend):

*   **Port Management:** Before starting a server, check for existing occupants: `lsof -i :<port>`.
*   **Background Persistence:** Use `nohup` and log redirection to ensure the process survives shell detachment:
    `nohup ./binary > process.log 2>&1 &`
*   **Verification:** Verify health after a short delay to catch immediate crashes:
    1.  `ps aux | grep binary`
    2.  `curl -v http://localhost:<port>/api/v1/sessions` (Check a known endpoint)
*   **Lifecycle Management:** Always `pkill <binary> || true` before rebuilding to avoid "address already in use" errors.
*   **Log Integrity:** Ensure log outputs intended for parsing are sanitized (e.g., strip trailing `<nil>` from Go's `%+v` output) to prevent UI or regex parsing errors.

## Meta-Testing & UI Verification

*   **Dogfooding:** Use Riptide itself to verify the Session Viewer UI. An agent should be able to:
    1.  Navigate to `http://localhost:8083`.
    2.  Verify the sidebar list is populated.
    3.  Click a session and confirm logs/images render correctly.
*   **Path Consistency:** Always verify that API prefixes (e.g., `/api/v1/`) are consistent between the backend router and frontend fetch calls.

### Web Application Debugging
*   **SPA Routing:** When serving a Single Page App from Go, verify that deep links (e.g., `/sessions/123`) do not return 404s. Ensure the backend has a "NotFound" handler that serves `index.html`.
*   **Console Hygiene:** When a UI "fails to load," checking the browser console is priority #1. Look for:
    *   **404s:** Mismatched API paths (`/sessions` vs `/api/v1/sessions`).
    *   **JS Errors:** "Circular structure to JSON" or "DefineForClassFields" errors (indicative of TS config mismatch).

- Work is NOT complete until `git push` succeeds

- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

