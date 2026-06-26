# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started, `bd prime` for full workflow context. Run `bd sync` to sync with git after pushing.

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

## Git & Remote Access

*   **SSH is blocked in this environment.** `git push` via SSH will hang indefinitely. Always switch to HTTPS before pushing, then restore:
    ```bash
    git remote set-url origin https://github.com/ghchinoy/riptide.git
    git push origin <branch>
    git remote set-url origin git@github.com:ghchinoy/riptide.git
    ```
*   **`gh pr create` may fail** with PAT scope restrictions. Fallback: merge directly into main with `git merge --no-ff <branch>` from the main worktree, then push.
*   **Worktrees for parallel work** — use `git worktree add ../<dir> -b <branch>` when another agent may be modifying the same files. Worktrees share the parent repo's remote config, so the HTTPS swap above applies from either worktree.

## SDK & Dependency Inspection

*   **Check the module cache, not docs.** To understand what types/fields a Go SDK version exposes (e.g., new `ThinkingConfig` fields, `ComputerUse.Environment`), grep the downloaded module source directly — it's faster and more accurate than web docs which may lag:
    ```bash
    grep -A10 "TypeName" $(go env GOPATH)/pkg/mod/google.golang.org/genai@vX.Y.Z/types.go
    ```
*   **Check latest SDK version** before starting any model-related work:
    ```bash
    go list -m -versions google.golang.org/genai | tr ' ' '\n' | tail -5
    ```

## Image Generation & Conversion

*   **NanoBanana MCP** generates PNG images to a local `output_directory`. Name the file deliberately in the prompt; the tool returns the saved path.
*   **Convert PNG→WebP** using `cwebp` (not `ffmpeg` — `libwebp` encoder is often absent):
    ```bash
    cwebp input.png -o output.webp -q 90
    ```

## Research & Terminology

*   **Don't limit research to Google/Cloud docs.** For current AI/ML terminology and concepts, arXiv is often more up-to-date and precise. A search like `arxiv.org/search/?query=agent+harness+llm&searchtype=all` will surface the latest papers using terms as they are actually being defined in the field.
*   **For genai model capability research**, the Google reference implementation repo (`github.com/google-gemini/computer-use-preview`) will often reflect the current recommended model name and patterns faster than documentation pages.


## Changelog Generation

To generate `CHANGELOG.md` from closed tasks:
```bash
bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | reverse | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
```

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:7510c1e2 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
