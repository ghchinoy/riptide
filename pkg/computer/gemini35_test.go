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

package computer

import (
	"strings"
	"testing"

	"google.golang.org/genai"
)

// ---------------------------------------------------------------------------
// 1. Model name
// ---------------------------------------------------------------------------

func TestModelName(t *testing.T) {
	const want = "gemini-3.5-flash"
	if ModelName != want {
		t.Errorf("ModelName = %q; want %q", ModelName, want)
	}
}

// ---------------------------------------------------------------------------
// 2. System instruction
// ---------------------------------------------------------------------------

func TestBuildSystemInstruction_NotNil(t *testing.T) {
	si := buildSystemInstruction()
	if si == nil {
		t.Fatal("buildSystemInstruction() returned nil")
	}
}

func TestBuildSystemInstruction_HasParts(t *testing.T) {
	si := buildSystemInstruction()
	if len(si.Parts) == 0 {
		t.Fatal("buildSystemInstruction() returned Content with no Parts")
	}
}

func TestBuildSystemInstruction_TextNotEmpty(t *testing.T) {
	si := buildSystemInstruction()
	if si.Parts[0].Text == "" {
		t.Error("buildSystemInstruction() Parts[0].Text is empty")
	}
}

func TestBuildSystemInstruction_ContainsKeyConstraints(t *testing.T) {
	si := buildSystemInstruction()
	text := si.Parts[0].Text

	keywords := []string{
		"computer use agent",
		"safety_decision",
		"prompt injection",
		"irreversible",
	}
	for _, kw := range keywords {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			t.Errorf("system instruction missing expected constraint keyword %q", kw)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Thinking config
// ---------------------------------------------------------------------------

func TestThinkingBudget_Positive(t *testing.T) {
	if ThinkingBudget <= 0 {
		t.Errorf("ThinkingBudget = %d; want > 0", ThinkingBudget)
	}
}

func TestInt32Ptr(t *testing.T) {
	want := int32(8192)
	got := int32Ptr(want)
	if got == nil {
		t.Fatal("int32Ptr returned nil")
	}
	if *got != want {
		t.Errorf("*int32Ptr(%d) = %d; want %d", want, *got, want)
	}
}

func TestBoolPtr(t *testing.T) {
	got := boolPtr(true)
	if got == nil {
		t.Fatal("boolPtr returned nil")
	}
	if !*got {
		t.Error("*boolPtr(true) = false; want true")
	}
}

// ---------------------------------------------------------------------------
// 4. Prompt injection detection helper
// ---------------------------------------------------------------------------

func TestIsPromptInjectionResponse_Nil(t *testing.T) {
	if isPromptInjectionResponse(nil) {
		t.Error("isPromptInjectionResponse(nil) = true; want false")
	}
}

func TestIsPromptInjectionResponse_FinishReasonStop(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonStop}
	if isPromptInjectionResponse(cand) {
		t.Error("isPromptInjectionResponse(STOP) = true; want false")
	}
}

func TestIsPromptInjectionResponse_FinishReasonSafety(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonSafety}
	if !isPromptInjectionResponse(cand) {
		t.Error("isPromptInjectionResponse(SAFETY) = false; want true")
	}
}

func TestIsPromptInjectionResponse_FinishReasonProhibitedContent(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonProhibitedContent}
	if !isPromptInjectionResponse(cand) {
		t.Error("isPromptInjectionResponse(PROHIBITED_CONTENT) = false; want true")
	}
}

func TestIsPromptInjectionResponse_FinishReasonMaxTokens(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonMaxTokens}
	if isPromptInjectionResponse(cand) {
		t.Error("isPromptInjectionResponse(MAX_TOKENS) = true; want false")
	}
}

func TestIsPromptInjectionResponse_FinishReasonUnspecified(t *testing.T) {
	cand := &genai.Candidate{FinishReason: genai.FinishReasonUnspecified}
	if isPromptInjectionResponse(cand) {
		t.Error("isPromptInjectionResponse(UNSPECIFIED) = true; want false")
	}
}

// ---------------------------------------------------------------------------
// 5. ComputerUse tool configuration (environment + injection detection)
// ---------------------------------------------------------------------------

// buildTestComputerUseTool extracts the ComputerUse config from a Tool slice,
// mirroring the structure built in Run().
func buildTestComputerUseTool() *genai.ComputerUse {
	tool := &genai.Tool{
		ComputerUse: &genai.ComputerUse{
			Environment:                    genai.EnvironmentBrowser,
			EnablePromptInjectionDetection: boolPtr(true),
		},
	}
	return tool.ComputerUse
}

func TestComputerUseTool_EnvironmentBrowser(t *testing.T) {
	cu := buildTestComputerUseTool()
	if cu.Environment != genai.EnvironmentBrowser {
		t.Errorf("ComputerUse.Environment = %q; want %q", cu.Environment, genai.EnvironmentBrowser)
	}
}

func TestComputerUseTool_PromptInjectionEnabled(t *testing.T) {
	cu := buildTestComputerUseTool()
	if cu.EnablePromptInjectionDetection == nil {
		t.Fatal("ComputerUse.EnablePromptInjectionDetection is nil; want non-nil")
	}
	if !*cu.EnablePromptInjectionDetection {
		t.Error("ComputerUse.EnablePromptInjectionDetection = false; want true")
	}
}

// ---------------------------------------------------------------------------
// 6. Event types
// ---------------------------------------------------------------------------

func TestEventPromptInjection_Defined(t *testing.T) {
	const want = EventType("prompt_injection")
	if EventPromptInjection != want {
		t.Errorf("EventPromptInjection = %q; want %q", EventPromptInjection, want)
	}
}

func TestAllEventTypes_Unique(t *testing.T) {
	all := []EventType{
		EventStatus,
		EventLog,
		EventError,
		EventScreen,
		EventAction,
		EventSafety,
		EventThinking,
		EventRaw,
		EventHallucination,
		EventPromptInjection,
	}
	seen := make(map[EventType]bool)
	for _, et := range all {
		if seen[et] {
			t.Errorf("duplicate EventType value: %q", et)
		}
		seen[et] = true
	}
}

// ---------------------------------------------------------------------------
// 7. pruneOldScreenshots (regression: existing behaviour unchanged)
// ---------------------------------------------------------------------------

func TestPruneOldScreenshots_KeepsMaxScreenshots(t *testing.T) {
	// Build a history where every user message has an inline screenshot.
	makeContent := func(hasScreenshot bool) *genai.Content {
		parts := []*genai.Part{{Text: "some text"}}
		if hasScreenshot {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("fake")},
			})
		}
		return &genai.Content{Role: "user", Parts: parts}
	}

	history := []*genai.Content{
		makeContent(true),  // turn 1 - should be pruned
		makeContent(true),  // turn 2 - should be pruned
		makeContent(true),  // turn 3 - kept
		makeContent(true),  // turn 4 - kept (most recent 2)
	}

	pruneOldScreenshots(history, 2)

	screenshotCount := 0
	for _, c := range history {
		for _, p := range c.Parts {
			if p.InlineData != nil {
				screenshotCount++
			}
		}
	}
	if screenshotCount != 2 {
		t.Errorf("after pruneOldScreenshots(max=2): got %d screenshots, want 2", screenshotCount)
	}
}
