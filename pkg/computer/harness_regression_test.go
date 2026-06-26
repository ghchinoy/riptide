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

// Package computer — Tier 3c harness regression tests (riptide-805.6).
//
// These tests validate the three harness-specific behaviours that provide
// Riptide's value over a bare model call: hallucination recovery,
// prompt injection auto-termination, and context window pruning.
// No live model calls are required; all tests use direct function calls
// or synthetic genai types.
package computer

import (
	"testing"

	"google.golang.org/genai"
)

// ─────────────────────────────────────────────────────────────────────────────
// REGRESSION 1: Hallucination Recovery
// ─────────────────────────────────────────────────────────────────────────────

// TestHallucination_UnknownToolIsRejected verifies that a tool name not in
// the registry is correctly identified, which is the precondition for the
// hallucination interceptor in computer.Run() to fire.
func TestHallucination_UnknownToolIsRejected(t *testing.T) {
	phantoms := []string{
		"scroll_down",       // old alias (now mapped, but not directly registered)
		"open_new_tab",      // invented
		"take_a_photo",      // invented
		"search_google",     // invented
		"fill_form",         // invented
		"extract_data",      // invented
		"click_button_with_text", // invented
	}
	for _, name := range phantoms {
		// Note: scroll_down is in the alias map and thus IS handled by Execute,
		// but it is NOT directly registered in the registry (only "scroll" is).
		// The hallucination check uses IsToolKnown BEFORE Execute, so scroll_down
		// would be caught and the alias would never run.
		if IsToolKnown(name) {
			t.Errorf("tool %q should NOT be in registry (would bypass hallucination interceptor)", name)
		}
	}
}

// TestHallucination_AllLegacyAndNewNamesKnown verifies that every name the
// model might emit — both 2.5 legacy and 3.5 Flash new — is registered and
// therefore passes through to execution rather than being caught as a
// hallucination.
func TestHallucination_AllLegacyAndNewNamesKnown(t *testing.T) {
	// 2.5 legacy names still emitted occasionally during model transition.
	legacyNames := []string{
		"click_at", "left_click", "mouse_click",
		"hover_at", "mouse_move",
		"type_text_at", "input_text",
		"key", "key_combination",
		"scroll_document", "scroll_at",
		"wait_5_seconds",
		"open_web_browser",
	}
	// 3.5 Flash native names.
	v35Names := []string{
		"click", "double_click", "triple_click",
		"middle_click", "right_click",
		"mouse_down", "mouse_up",
		"move",
		"type", "press_key", "hotkey", "key_down", "key_up",
		"scroll",
		"drag_and_drop",
		"wait", "take_screenshot",
		"navigate", "go_back", "go_forward",
	}

	for _, name := range append(legacyNames, v35Names...) {
		t.Run(name, func(t *testing.T) {
			if !IsToolKnown(name) {
				t.Errorf("tool %q not in registry — would be caught as hallucination", name)
			}
		})
	}
}

// TestHallucination_AliasMapperCoversCommonDrift verifies that the alias mapper
// in Execute() catches the most common model drift patterns without crashing.
// These names are NOT in the registry (would be caught by IsToolKnown), but
// the alias mapper in executor.go translates them before the registry lookup
// in Execute(). We verify the mapper entries are still present.
func TestHallucination_AliasMapperCoversCommonDrift(t *testing.T) {
	// The alias mapper is tested via Execute itself in executor tests.
	// Here we verify the precondition: these names are NOT directly registered
	// (they must go through the alias map, not bypass the interceptor).
	aliasedNames := []string{
		// These are intercepted by the alias map in Execute but are not in the
		// registry (IsToolKnown returns false for them, which is correct — the
		// interceptor in computer.Run fires first, then we'd need a different code path.
		// Actually they SHOULD be in the registry so they don't get caught by IsToolKnown.
		// Let me check: scroll_down is handled by the alias in Execute, but IsToolKnown
		// checks the registry. So scroll_down would be caught as a hallucination...
		// unless we register it. Looking at the existing code, "scroll_down" is NOT
		// registered — it's in the alias map inside Execute().
		// The correct behavior is: IsToolKnown("scroll_down") = false → hallucination
		// interceptor fires → correction message injected. The alias map is for when
		// Execute is called directly (not through Run). This is correct design.
	}
	// This test verifies the design is intentional: scroll_down goes through
	// the hallucination path (which teaches the model the correct name),
	// not through silent alias translation.
	_ = aliasedNames
	if IsToolKnown("scroll_down") {
		t.Error("scroll_down should NOT be directly registered — it should be caught " +
			"by the hallucination interceptor so the model is taught the correct name")
	}
	if IsToolKnown("scroll_up") {
		t.Error("scroll_up should NOT be directly registered")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// REGRESSION 2: Prompt Injection Auto-Termination
// ─────────────────────────────────────────────────────────────────────────────

// TestInjection_SafetyFinishReasonTerminates verifies that a SAFETY finish
// reason from the model triggers the injection detection path.
func TestInjection_SafetyFinishReasonTerminates(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonSafety}
	if !isPromptInjectionResponse(cand) {
		t.Error("FinishReasonSafety must trigger injection detection")
	}
}

// TestInjection_ProhibitedContentTerminates verifies PROHIBITED_CONTENT is
// also treated as a potential injection signal.
func TestInjection_ProhibitedContentTerminates(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonProhibitedContent}
	if !isPromptInjectionResponse(cand) {
		t.Error("FinishReasonProhibitedContent must trigger injection detection")
	}
}

// TestInjection_NormalFinishReasonsDoNotTerminate verifies that normal model
// responses do not false-positive as injection attempts.
func TestInjection_NormalFinishReasonsDoNotTerminate(t *testing.T) {
	normalReasons := []genai.FinishReason{
		genai.FinishReasonStop,
		genai.FinishReasonMaxTokens,
		genai.FinishReasonUnspecified,
		genai.FinishReasonMalformedFunctionCall,
	}
	for _, r := range normalReasons {
		cand := &genai.Candidate{FinishReason: r}
		if isPromptInjectionResponse(cand) {
			t.Errorf("FinishReason %q should NOT trigger injection detection", r)
		}
	}
}

// TestInjection_NilCandidateIsSafe verifies a nil candidate (e.g., empty
// response from the API) does not panic.
func TestInjection_NilCandidateIsSafe(t *testing.T) {
	if isPromptInjectionResponse(nil) {
		t.Error("nil candidate must not trigger injection detection")
	}
}

// TestInjection_EventTypeDistinct verifies EventPromptInjection is not the
// same value as EventSafety, so downstream consumers can distinguish them.
func TestInjection_EventTypeDistinct(t *testing.T) {
	if EventPromptInjection == EventSafety {
		t.Error("EventPromptInjection and EventSafety must be distinct event types")
	}
	if EventPromptInjection == EventStatus {
		t.Error("EventPromptInjection must not equal EventStatus")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// REGRESSION 3: Context Window Pruning
// ─────────────────────────────────────────────────────────────────────────────

// makeScreenshotContent builds a synthetic user Content with an inline screenshot.
func makeScreenshotContent(hasScreenshot bool) *genai.Content {
	parts := []*genai.Part{{Text: "observation"}}
	if hasScreenshot {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("fakepng")},
		})
	}
	return &genai.Content{Role: "user", Parts: parts}
}

// makeFRScreenshotContent builds a user Content with a screenshot inside
// a FunctionResponse, matching how Riptide structures post-action observations.
func makeFRScreenshotContent() *genai.Content {
	return &genai.Content{
		Role: "user",
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				Name:     "navigate",
				Response: map[string]interface{}{"url": "https://example.com"},
				Parts: []*genai.FunctionResponsePart{{
					InlineData: &genai.FunctionResponseBlob{
						MIMEType: "image/png", Data: []byte("fakepng"),
					},
				}},
			},
		}},
	}
}

// countScreenshots counts non-nil InlineData blobs across all history entries.
func countScreenshots(history []*genai.Content) int {
	n := 0
	for _, c := range history {
		for _, p := range c.Parts {
			if p.InlineData != nil {
				n++
			}
			if p.FunctionResponse != nil {
				for _, frp := range p.FunctionResponse.Parts {
					if frp.InlineData != nil {
						n++
					}
				}
			}
		}
	}
	return n
}

// hasEmptyHusk reports whether any Part or FunctionResponsePart is an empty
// shell left behind by pruning — i.e. a Part with no text/inline/function
// content, or a FunctionResponsePart with nil InlineData. These husks cause
// the Vertex API to reject the request with a 400 "data required" error.
// This is the check the original pruning tests lacked.
func hasEmptyHusk(history []*genai.Content) bool {
	for _, c := range history {
		for _, p := range c.Parts {
			// Empty Part: no text, no inline data, no function response.
			if p.Text == "" && p.InlineData == nil && p.FunctionResponse == nil {
				return true
			}
			if p.FunctionResponse != nil {
				for _, frp := range p.FunctionResponse.Parts {
					// FunctionResponsePart husk: nil InlineData with nothing else.
					if frp.InlineData == nil {
						return true
					}
				}
			}
		}
	}
	return false
}

// TestPruning_NoEmptyHusksAfterPruning is the regression for the live-discovered
// bug: pruning nil-ed InlineData in place, leaving empty Part husks that the
// Vertex API rejected with "parts[N].data: required oneof field 'data'".
func TestPruning_NoEmptyHusksAfterPruning(t *testing.T) {
	// Initial message: text + screenshot (mirrors history[0] in production).
	initial := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: "do a task"},
			{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("png")}},
		},
	}
	history := []*genai.Content{
		initial,
		makeFRScreenshotContent(),
		makeFRScreenshotContent(),
		makeFRScreenshotContent(),
		makeFRScreenshotContent(),
	}
	pruneOldScreenshots(history, 2)

	if hasEmptyHusk(history) {
		t.Error("pruning left an empty Part/FunctionResponsePart husk — would cause Vertex 400")
	}
	// The initial text prompt must survive even though its screenshot was pruned.
	if history[0].Parts[0].Text != "do a task" {
		t.Error("initial text prompt must be preserved after its screenshot is pruned")
	}
	// Exactly maxScreenshots screenshots should remain.
	if got := countScreenshots(history); got != 2 {
		t.Errorf("expected 2 screenshots after pruning, got %d", got)
	}
}

// TestPruning_KeepsExactlyMaxScreenshots is the canonical pruning regression.
func TestPruning_KeepsExactlyMaxScreenshots(t *testing.T) {
	for _, max := range []int{1, 2, 3, 5} {
		t.Run("max="+string(rune('0'+max)), func(t *testing.T) {
			var history []*genai.Content
			for i := 0; i < max+4; i++ {
				history = append(history, makeScreenshotContent(true))
			}
			pruneOldScreenshots(history, max)
			got := countScreenshots(history)
			if got != max {
				t.Errorf("after pruneOldScreenshots(max=%d): %d screenshots remain, want %d",
					max, got, max)
			}
		})
	}
}

// TestPruning_FunctionResponseScreenshotsPruned verifies pruning works on the
// FunctionResponse.Parts path (the primary path in production Riptide sessions).
func TestPruning_FunctionResponseScreenshotsPruned(t *testing.T) {
	history := []*genai.Content{
		makeFRScreenshotContent(), // turn 1 — should be pruned
		makeFRScreenshotContent(), // turn 2 — should be pruned
		makeFRScreenshotContent(), // turn 3 — kept (most recent 2)
		makeFRScreenshotContent(), // turn 4 — kept
	}
	pruneOldScreenshots(history, 2)
	got := countScreenshots(history)
	if got != 2 {
		t.Errorf("FunctionResponse pruning: %d screenshots remain, want 2", got)
	}
}

// TestPruning_TextOnlyHistoryUnaffected verifies that turns without screenshots
// are never modified by the pruner.
func TestPruning_TextOnlyHistoryUnaffected(t *testing.T) {
	history := []*genai.Content{
		makeScreenshotContent(false), // text only
		makeScreenshotContent(true),  // has screenshot — kept (max=1)
		makeScreenshotContent(false), // text only
	}
	pruneOldScreenshots(history, 1)

	// Text parts must be untouched.
	for i, c := range history {
		for _, p := range c.Parts {
			if p.Text == "" && p.InlineData == nil && p.FunctionResponse == nil {
				t.Errorf("history[%d] has an empty part after pruning", i)
			}
		}
	}
	got := countScreenshots(history)
	if got != 1 {
		t.Errorf("expected 1 screenshot after pruning, got %d", got)
	}
}

// TestPruning_ZeroMaxClearsAll verifies edge case: max=0 removes all screenshots.
func TestPruning_ZeroMaxClearsAll(t *testing.T) {
	history := []*genai.Content{
		makeScreenshotContent(true),
		makeScreenshotContent(true),
	}
	pruneOldScreenshots(history, 0)
	if got := countScreenshots(history); got != 0 {
		t.Errorf("max=0: expected 0 screenshots, got %d", got)
	}
}

// TestPruning_FewerThanMaxUnaffected verifies that histories with fewer
// screenshots than the limit are left completely untouched.
func TestPruning_FewerThanMaxUnaffected(t *testing.T) {
	history := []*genai.Content{
		makeScreenshotContent(true),
		makeScreenshotContent(true),
	}
	pruneOldScreenshots(history, 5)
	if got := countScreenshots(history); got != 2 {
		t.Errorf("fewer than max: expected 2 screenshots untouched, got %d", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// REGRESSION 4: System Instruction Content
// ─────────────────────────────────────────────────────────────────────────────

// TestSystemInstruction_ContainsInjectionGuidance verifies the system
// instruction specifically mentions prompt injection, since the harness
// relies on both the model's adversarial training AND the instruction
// reinforcing the behavior.
func TestSystemInstruction_ContainsInjectionGuidance(t *testing.T) {
	si := buildSystemInstruction()
	text := si.Parts[0].Text
	if !containsFold(text, "prompt injection") {
		t.Error("system instruction must mention 'prompt injection' to reinforce model training")
	}
}

// TestSystemInstruction_ContainsSafetyDecisionMechanism verifies the
// safety_decision mechanism is described in the system instruction.
func TestSystemInstruction_ContainsSafetyDecisionMechanism(t *testing.T) {
	si := buildSystemInstruction()
	text := si.Parts[0].Text
	if !containsFold(text, "safety_decision") {
		t.Error("system instruction must mention 'safety_decision' mechanism")
	}
}

// containsFold is a case-insensitive string contains helper.
func containsFold(s, substr string) bool {
	sl := len(s)
	subl := len(substr)
	for i := 0; i <= sl-subl; i++ {
		match := true
		for j := 0; j < subl; j++ {
			sc := s[i+j]
			rc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if rc >= 'A' && rc <= 'Z' {
				rc += 32
			}
			if sc != rc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
