// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package results

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseSessionLog_GoalAchieved(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: Search for Gemini on Google <nil>
2026/06/26 10:00:01 [status] Turn 1/10: Sending request... <nil>
2026/06/26 10:00:02 [action] Tool Call: navigate <nil>
2026/06/26 10:00:03 [status] Turn 2/10: Sending request... <nil>
2026/06/26 10:00:04 [action] Tool Call: click <nil>
2026/06/26 10:00:05 [status] Goal Achieved. <nil>
2026/06/26 10:00:05 [status] Session Finished. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != OutcomeGoalAchieved {
		t.Errorf("Outcome = %q; want %q", r.Outcome, OutcomeGoalAchieved)
	}
	if r.Turns != 2 {
		t.Errorf("Turns = %d; want 2", r.Turns)
	}
	if len(r.Actions) != 2 {
		t.Errorf("Actions = %d; want 2", len(r.Actions))
	}
	if r.Prompt != "Search for Gemini on Google" {
		t.Errorf("Prompt = %q; want 'Search for Gemini on Google'", r.Prompt)
	}
}

func TestParseSessionLog_MaxTurns(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: Do something complex <nil>
2026/06/26 10:00:01 [status] Turn 5/5: Sending request... <nil>
2026/06/26 10:00:02 [action] Tool Call: scroll <nil>
2026/06/26 10:00:03 [status] Max Turns Reached. <nil>
2026/06/26 10:00:03 [status] Session Finished. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != OutcomeMaxTurns {
		t.Errorf("Outcome = %q; want %q", r.Outcome, OutcomeMaxTurns)
	}
	if r.Turns != 5 {
		t.Errorf("Turns = %d; want 5", r.Turns)
	}
}

func TestParseSessionLog_HallucinationCounting(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: test <nil>
2026/06/26 10:00:01 [hallucination] Hallucinated Tool: scroll_down <nil>
2026/06/26 10:00:02 [hallucination] Hallucinated Tool: go_left <nil>
2026/06/26 10:00:03 [status] Goal Achieved. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.Hallucinations != 2 {
		t.Errorf("Hallucinations = %d; want 2", r.Hallucinations)
	}
}

func TestParseSessionLog_PromptInjection(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: visit example.com <nil>
2026/06/26 10:00:01 [status] Turn 1/10: Sending request... <nil>
2026/06/26 10:00:02 [prompt_injection] Prompt injection detected (FinishReason: SAFETY). Terminating. <nil>
2026/06/26 10:00:02 [status] Session Finished. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != OutcomeInjection {
		t.Errorf("Outcome = %q; want %q", r.Outcome, OutcomeInjection)
	}
	if !r.InjectionFired {
		t.Error("InjectionFired should be true")
	}
}

func TestParseSessionLog_SafetyDenied(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: delete my account <nil>
2026/06/26 10:00:01 [safety] Safety Decision Required <nil>
2026/06/26 10:00:02 [status] User denied safety request. Terminating. <nil>
2026/06/26 10:00:02 [status] Session Finished. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != OutcomeSafety {
		t.Errorf("Outcome = %q; want %q", r.Outcome, OutcomeSafety)
	}
	if !r.SafetyFired {
		t.Error("SafetyFired should be true")
	}
}

func TestParseSessionLog_Error(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: test <nil>
2026/06/26 10:00:01 [error] Model call failed: context deadline exceeded <nil>
2026/06/26 10:00:01 [status] Session Finished. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != OutcomeError {
		t.Errorf("Outcome = %q; want %q", r.Outcome, OutcomeError)
	}
	if r.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty")
	}
}

func TestParseSessionLog_CompleteReportsCorrectly(t *testing.T) {
	complete := `2026/06/26 10:00:00 [status] Goal Achieved.`
	incomplete := `2026/06/26 10:00:00 [log] Prompt: test`

	r1, _ := ParseSessionLog(writeLog(t, complete))
	if !r1.Complete() {
		t.Error("GoalAchieved session should be Complete()")
	}

	r2, _ := ParseSessionLog(writeLog(t, incomplete))
	if r2.Complete() {
		t.Error("Session with no terminal event should not be Complete()")
	}
}

func TestParseSessionLog_TokenUsage(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: search task <nil>
2026/06/26 10:00:01 [status] Turn 1/10: Sending request... <nil>
2026/06/26 10:00:02 [log] Tokens: prompt=1500 candidates=200 thoughts=300 total=2000 cached=0 <nil>
2026/06/26 10:00:03 [status] Turn 2/10: Sending request... <nil>
2026/06/26 10:00:04 [log] Tokens: prompt=1800 candidates=150 thoughts=250 total=2200 cached=500 <nil>
2026/06/26 10:00:05 [status] Goal Achieved. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	// Tokens accumulate across both turns.
	if r.PromptTokens != 3300 {
		t.Errorf("PromptTokens = %d; want 3300", r.PromptTokens)
	}
	if r.CandidateTokens != 350 {
		t.Errorf("CandidateTokens = %d; want 350", r.CandidateTokens)
	}
	if r.ThoughtTokens != 550 {
		t.Errorf("ThoughtTokens = %d; want 550", r.ThoughtTokens)
	}
	if r.TotalTokens != 4200 {
		t.Errorf("TotalTokens = %d; want 4200", r.TotalTokens)
	}
	if r.CachedTokens != 500 {
		t.Errorf("CachedTokens = %d; want 500", r.CachedTokens)
	}
}

func TestParseSessionLog_SafetyDecisionCount(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: risky task <nil>
2026/06/26 10:00:01 [safety] Safety Decision Required <nil>
2026/06/26 10:00:02 [safety] Safety Decision Required <nil>
2026/06/26 10:00:03 [status] Goal Achieved. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.SafetyDecisions != 2 {
		t.Errorf("SafetyDecisions = %d; want 2", r.SafetyDecisions)
	}
	if !r.SafetyFired {
		t.Error("SafetyFired should be true")
	}
}

func TestParseSessionLog_NoTokensIsZero(t *testing.T) {
	log := `2026/06/26 10:00:00 [log] Prompt: legacy session without token logging <nil>
2026/06/26 10:00:01 [status] Goal Achieved. <nil>`

	r, err := ParseSessionLog(writeLog(t, log))
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d; want 0 for session without token logs", r.TotalTokens)
	}
}

func TestParseSessionDir(t *testing.T) {
	base := t.TempDir()

	// Session 1: goal achieved
	s1 := filepath.Join(base, "session-aaa")
	_ = os.MkdirAll(s1, 0755)
	_ = os.WriteFile(filepath.Join(s1, "session.log"),
		[]byte(`2026/06/26 10:00:00 [log] Prompt: task one
2026/06/26 10:00:01 [status] Turn 1/10: Sending request...
2026/06/26 10:00:02 [status] Goal Achieved.`), 0644)

	// Session 2: max turns
	s2 := filepath.Join(base, "session-bbb")
	_ = os.MkdirAll(s2, 0755)
	_ = os.WriteFile(filepath.Join(s2, "session.log"),
		[]byte(`2026/06/26 10:00:00 [log] Prompt: task two
2026/06/26 10:00:01 [status] Turn 3/3: Sending request...
2026/06/26 10:00:02 [status] Max Turns Reached.`), 0644)

	// Non-session dir (no log)
	_ = os.MkdirAll(filepath.Join(base, "not-a-session"), 0755)

	results, err := ParseSessionDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("ParseSessionDir: got %d results, want 2", len(results))
	}

	byID := make(map[string]*SessionResult)
	for _, r := range results {
		byID[r.SessionID] = r
	}

	if byID["session-aaa"].Outcome != OutcomeGoalAchieved {
		t.Errorf("session-aaa outcome = %q; want goal_achieved", byID["session-aaa"].Outcome)
	}
	if byID["session-bbb"].Outcome != OutcomeMaxTurns {
		t.Errorf("session-bbb outcome = %q; want max_turns", byID["session-bbb"].Outcome)
	}
}
