# Tier 1 Validation — Canonical Smoke Tests

Five scenarios drawn directly from the reference material in `sources/`.
Each maps to a task demonstrated in the Google Colab intro notebook or the
`computer-use-preview` reference implementation.

Pass criteria: the session ends with `Goal Achieved.` or the stated
observable outcome is present in the final screenshot / session log.

---

## How to run

```bash
# Build first
make build-agent

# Run a scenario (replace N with scenario number)
bin/riptide run --prompt "<PROMPT>" --max-turns 15 --axt

# Inspect results
bin/riptide sessions list
bin/riptide sessions show <session-id>
```

Record the session ID, outcome, and turn count in `tier1_results.md`.

---

## Scenario 1 — Direct URL Navigation
**Source:** intro notebook, step-by-step walkthrough

**Prompt:**
```
Navigate directly to https://flights.google.com and take a screenshot of the landing page.
```

**Pass criteria:**
- `Goal Achieved.` in session log
- Final screenshot shows Google Flights UI
- Turn count ≤ 3

**Rationale:** The simplest possible task — a single `navigate` call. Validates
the core Observe→Reason→Act loop end-to-end with the 3.5 Flash model.

---

## Scenario 2 — Google Search
**Source:** `sources/computer-use-preview/agent.py`, agent loop example

**Prompt:**
```
Go to https://www.google.com and search for "Gemini 3.5 Flash computer use". 
Report the title of the first result you see.
```

**Pass criteria:**
- `Goal Achieved.` in session log
- Session log contains a `thinking` entry with a title from the results page
- Turn count ≤ 5

**Rationale:** Tests navigation + text input + scroll/observation. Mirrors the
standard "Hello World" task in both the intro notebook and reference impl.

---

## Scenario 3 — Google Store Category Navigation
**Source:** `sources/computer-use-preview/main.py`, default demo prompt

**Prompt:**
```
Navigate to the Google Store and find the Pixel phones category. 
Tell me how many Pixel phone models are currently listed.
```

**Pass criteria:**
- `Goal Achieved.` in session log
- Model reports a count of Pixel phone models
- Turn count ≤ 8

**Rationale:** Multi-step navigation requiring the model to click through to a
category page and extract information. The exact prompt used in the Python
reference implementation's README.

---

## Scenario 4 — Flight Search (Multi-step Form)
**Source:** intro notebook, agent loop section

**Prompt:**
```
Find me a flight from San Francisco (SFO) to Honolulu (HNL) departing next 
Monday, returning the following Friday. Start at https://flights.google.com
and tell me the cheapest round-trip option you find.
```

**Pass criteria:**
- `Goal Achieved.` in session log
- Model reports a price and/or airline
- Turn count ≤ 15

**Rationale:** The canonical multi-step computer use task from the intro notebook.
Requires navigation, date selection, form interaction, and result extraction.
Tests the full range of Riptide's tool set under a realistic task.

---

## Scenario 5 — Controlled Form Fill (testserver)
**Source:** `cmd/testserver` — no external network required

```bash
# Start testserver first
go run cmd/testserver/main.go &
```

**Prompt:**
```
Go to http://localhost:8080, enter "Agent Smith" as the name in the form 
field, and click the Submit button. Tell me what confirmation message appears.
```

**Pass criteria:**
- `Goal Achieved.` in session log
- Model reports the confirmation message from the testserver response
- Turn count ≤ 5
- No hallucinations in session log

**Rationale:** Fully controlled, repeatable, no network variance. Designed to be
run in CI. Validates form fill + submit with deterministic success verification.
The only scenario that can be automated without live Vertex AI credentials
(for smoke-test purposes the model call is still needed, but the environment
is deterministic).

---

## Results template

Copy to `tier1_results.md` after running:

```
| Scenario | Session ID | Outcome | Turns | Hallucinations | Notes |
|----------|-----------|---------|-------|----------------|-------|
| 1. URL Nav   | | | | | |
| 2. Search    | | | | | |
| 3. Store Nav | | | | | |
| 4. Flights   | | | | | |
| 5. Testserver| | | | | |
```
