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

// Package results parses Riptide session logs into structured metrics.
// It is the foundation for Tier 2 (harness-vs-bare comparison) and Tier 3c
// (regression test) automated result extraction.
package results

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Outcome describes how a session ended.
type Outcome string

const (
	OutcomeGoalAchieved  Outcome = "goal_achieved"
	OutcomeMaxTurns      Outcome = "max_turns"
	OutcomeInjection     Outcome = "prompt_injection"
	OutcomeSafety        Outcome = "safety_denied"
	OutcomeError         Outcome = "error"
	OutcomeUnknown       Outcome = "unknown"
)

// SessionResult is the parsed, structured summary of a single Riptide session.
type SessionResult struct {
	SessionID      string        `json:"session_id"`
	Prompt         string        `json:"prompt"`
	Outcome        Outcome       `json:"outcome"`
	Turns          int           `json:"turns"`
	Hallucinations int           `json:"hallucinations"`
	Actions        []string      `json:"actions"`
	FinalURL       string        `json:"final_url"`
	Duration       time.Duration `json:"duration_ms"`
	InjectionFired bool          `json:"injection_fired"`
	SafetyFired    bool          `json:"safety_fired"`
	ErrorMessage   string        `json:"error_message,omitempty"`
}

// Complete reports whether the session reached a definitive terminal state.
func (r *SessionResult) Complete() bool {
	return r.Outcome != OutcomeUnknown
}

// ParseSessionLog reads a session.log file and extracts structured metrics.
func ParseSessionLog(path string) (*SessionResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	res := &SessionResult{Outcome: OutcomeUnknown}

	var (
		promptRe     = regexp.MustCompile(`\[log\] Prompt: (.+)`)
		turnRe       = regexp.MustCompile(`\[status\] Turn (\d+)/`)
		actionRe     = regexp.MustCompile(`\[action\] Tool Call: (.+)`)
		halluRe      = regexp.MustCompile(`\[hallucination\]`)
		urlRe        = regexp.MustCompile(`\[log\].*url[=:]\s*(https?://\S+)`)
		errorRe      = regexp.MustCompile(`\[error\] (.+)`)
		injectionRe  = regexp.MustCompile(`\[prompt_injection\]`)
		safetyRe     = regexp.MustCompile(`\[safety\]`)
	)

	scanner := bufio.NewScanner(f)
	maxTurn := 0
	for scanner.Scan() {
		line := scanner.Text()

		if m := promptRe.FindStringSubmatch(line); len(m) > 1 {
			res.Prompt = strings.TrimSuffix(m[1], " <nil>")
		}
		if m := turnRe.FindStringSubmatch(line); len(m) > 1 {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			if n > maxTurn {
				maxTurn = n
			}
		}
		if m := actionRe.FindStringSubmatch(line); len(m) > 1 {
			res.Actions = append(res.Actions, strings.TrimSuffix(m[1], " <nil>"))
		}
		if halluRe.MatchString(line) {
			res.Hallucinations++
		}
		if injectionRe.MatchString(line) {
			res.InjectionFired = true
			res.Outcome = OutcomeInjection
		}
		if safetyRe.MatchString(line) {
			res.SafetyFired = true
		}
		if m := urlRe.FindStringSubmatch(line); len(m) > 1 {
			res.FinalURL = m[1]
		}
		if m := errorRe.FindStringSubmatch(line); len(m) > 1 {
			res.ErrorMessage = strings.TrimSuffix(m[1], " <nil>")
			if res.Outcome == OutcomeUnknown {
				res.Outcome = OutcomeError
			}
		}

		// Terminal state detection — last one wins.
		switch {
		case strings.Contains(line, "Goal Achieved."):
			res.Outcome = OutcomeGoalAchieved
		case strings.Contains(line, "Max Turns Reached."):
			res.Outcome = OutcomeMaxTurns
		case strings.Contains(line, "User denied safety request"):
			res.Outcome = OutcomeSafety
		}
	}

	res.Turns = maxTurn
	if res.Outcome == OutcomeUnknown && res.ErrorMessage != "" {
		res.Outcome = OutcomeError
	}
	return res, scanner.Err()
}

// ParseSessionDir reads all sessions under dir and returns one result per session.
// Sessions without a session.log are skipped silently.
func ParseSessionDir(dir string) ([]*SessionResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*SessionResult
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		logPath := dir + "/" + e.Name() + "/session.log"
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			continue
		}
		r, err := ParseSessionLog(logPath)
		if err != nil {
			continue
		}
		r.SessionID = e.Name()
		out = append(out, r)
	}
	return out, nil
}
