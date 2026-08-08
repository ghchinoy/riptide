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
	SafetyDecisions int          `json:"safety_decisions"`
	ErrorMessage   string        `json:"error_message,omitempty"`

	// Token usage aggregated across all turns. Populated from the
	// "[log] Tokens:" events emitted by computer.go each turn.
	PromptTokens    int `json:"prompt_tokens"`
	CandidateTokens int `json:"candidate_tokens"`
	ThoughtTokens   int `json:"thought_tokens"`
	TotalTokens     int `json:"total_tokens"`
	CachedTokens    int `json:"cached_tokens"`

	// Wall-time metrics derived from log timestamps.
	// WallSeconds is the elapsed time from first to last log line.
	// ModelSeconds is the sum of "Model response received in Xs" durations.
	// HarnessSeconds is WallSeconds − ModelSeconds (overhead: screenshots, waits, AXTree).
	// TurnWallSeconds contains the per-turn wall time derived from inter-turn timestamps.
	WallSeconds      float64   `json:"wall_seconds"`
	ModelSeconds     float64   `json:"model_seconds"`
	HarnessSeconds   float64   `json:"harness_seconds"`
	TurnWallSeconds  []float64 `json:"turn_wall_seconds,omitempty"`
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
		promptRe    = regexp.MustCompile(`\[log\] Prompt: (.+)`)
		turnRe      = regexp.MustCompile(`\[status\] Turn (\d+)/`)
		actionRe    = regexp.MustCompile(`\[action\] Tool Call: (.+)`)
		halluRe     = regexp.MustCompile(`\[hallucination\]`)
		urlRe       = regexp.MustCompile(`\[log\].*url[=:]\s*(https?://\S+)`)
		errorRe     = regexp.MustCompile(`\[error\] (.+)`)
		injectionRe = regexp.MustCompile(`\[prompt_injection\]`)
		safetyRe    = regexp.MustCompile(`\[safety\]`)
		tokensRe    = regexp.MustCompile(`\[log\] Tokens: prompt=(\d+) candidates=(\d+) thoughts=(\d+) total=(\d+) cached=(\d+)`)
		// Model response time: "Model response received in 6.369s"
		modelTimeRe = regexp.MustCompile(`Model response received in ([\d.]+)([a-zµ]+)`)
		// Log line timestamp prefix: "2026/06/26 12:05:51"
		tsRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)
	)
	const tsLayout = "2006/01/02 15:04:05"

	var (
		firstTS   time.Time
		lastTS    time.Time
		turnTS    time.Time // timestamp of current turn-start
		inTurn    bool
	)

	scanner := bufio.NewScanner(f)
	maxTurn := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Parse wall-time from log timestamp prefix.
		if m := tsRe.FindStringSubmatch(line); len(m) > 1 {
			if t, err := time.Parse(tsLayout, m[1]); err == nil {
				if firstTS.IsZero() {
					firstTS = t
				}
				lastTS = t
			}
		}

		// Track per-turn wall time: measure from Turn N start to Turn N+1 start.
		if m := turnRe.FindStringSubmatch(line); len(m) > 1 {
			if inTurn && !turnTS.IsZero() {
				if m2 := tsRe.FindStringSubmatch(line); len(m2) > 1 {
					if t, err := time.Parse(tsLayout, m2[1]); err == nil {
						res.TurnWallSeconds = append(res.TurnWallSeconds, t.Sub(turnTS).Seconds())
						turnTS = t
					}
				}
			} else {
				if m2 := tsRe.FindStringSubmatch(line); len(m2) > 1 {
					if t, err := time.Parse(tsLayout, m2[1]); err == nil {
						turnTS = t
						inTurn = true
					}
				}
			}
		}

		// Accumulate model response time from log messages.
		if m := modelTimeRe.FindStringSubmatch(line); len(m) == 3 {
			var val float64
			fmt.Sscanf(m[1], "%f", &val)
			switch m[2] {
			case "ms":
				res.ModelSeconds += val / 1000
			case "s":
				res.ModelSeconds += val
			case "µs":
				res.ModelSeconds += val / 1e6
			}
		}

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
			res.SafetyDecisions++
		}
		if m := tokensRe.FindStringSubmatch(line); len(m) == 6 {
			var p, c, th, tot, ca int
			fmt.Sscanf(m[1], "%d", &p)
			fmt.Sscanf(m[2], "%d", &c)
			fmt.Sscanf(m[3], "%d", &th)
			fmt.Sscanf(m[4], "%d", &tot)
			fmt.Sscanf(m[5], "%d", &ca)
			res.PromptTokens += p
			res.CandidateTokens += c
			res.ThoughtTokens += th
			res.TotalTokens += tot
			res.CachedTokens += ca
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
		case strings.Contains(line, "User denied safety request"),
			strings.Contains(line, "No SafetyHandler registered"):
			res.Outcome = OutcomeSafety
		case strings.Contains(line, "blockReason") && strings.Contains(line, "SAFETY"),
			strings.Contains(line, "No candidates returned"):
			// Prompt-level safety block: model returned 0 candidates.
			// Only set if not already resolved to a more specific terminal state.
			if res.Outcome == OutcomeUnknown {
				res.Outcome = OutcomeSafety
				res.SafetyFired = true
			}
		}
	}

	res.Turns = maxTurn
	if res.Outcome == OutcomeUnknown && res.ErrorMessage != "" {
		res.Outcome = OutcomeError
	}
	// Compute wall-time summary.
	if !firstTS.IsZero() && !lastTS.IsZero() {
		res.WallSeconds = lastTS.Sub(firstTS).Seconds()
		res.HarnessSeconds = res.WallSeconds - res.ModelSeconds
		if res.HarnessSeconds < 0 {
			res.HarnessSeconds = 0
		}
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
