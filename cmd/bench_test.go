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

package cmd

import (
	"strings"
	"testing"

	"github.com/ghchinoy/riptide/pkg/results"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"   ", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{"  gemini-3.5-flash , gemini-3.6-flash  ", []string{"gemini-3.5-flash", "gemini-3.6-flash"}},
		{"MINIMAL, LOW, , HIGH", []string{"MINIMAL", "LOW", "HIGH"}},
	}

	for _, tt := range tests {
		got := splitAndTrim(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitAndTrim(%q) len = %d; want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitAndTrim(%q)[%d] = %q; want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestGenerateBenchmarkReport(t *testing.T) {
	prompt := "Test task"
	maxTurns := 5
	runResults := []BenchmarkRunResult{
		{
			Item: BenchmarkConfigItem{
				Model:         "gemini-3.5-flash",
				ThinkingLevel: "LOW",
			},
			SessionID:      "test-session-1",
			Outcome:        results.OutcomeGoalAchieved,
			Turns:          3,
			WallSeconds:    12.5,
			ModelSeconds:   10.0,
			AvgTurnLatency: 4.16,
			ThoughtTokens:  500,
			TotalTokens:    2000,
		},
		{
			Item: BenchmarkConfigItem{
				Model:          "gemini-3.6-flash",
				ThinkingBudget: 2048,
			},
			SessionID:      "test-session-2",
			Outcome:        results.OutcomeMaxTurns,
			Turns:          5,
			WallSeconds:    20.0,
			ModelSeconds:   18.0,
			AvgTurnLatency: 4.0,
			ThoughtTokens:  800,
			TotalTokens:    3500,
		},
	}

	report := generateBenchmarkReport(prompt, maxTurns, runResults)

	if !strings.Contains(report, "Riptide Model & Thinking Config Benchmark Report") {
		t.Errorf("generateBenchmarkReport missing header")
	}
	if !strings.Contains(report, "gemini-3.5-flash") || !strings.Contains(report, "gemini-3.6-flash") {
		t.Errorf("generateBenchmarkReport missing model names")
	}
	if !strings.Contains(report, "LOW") || !strings.Contains(report, "Budget=2048") {
		t.Errorf("generateBenchmarkReport missing thinking config descriptions")
	}
	if !strings.Contains(report, "goal_achieved") || !strings.Contains(report, "max_turns") {
		t.Errorf("generateBenchmarkReport missing outcomes")
	}
}
