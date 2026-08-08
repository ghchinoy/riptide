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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ghchinoy/riptide/pkg/computer"
	"github.com/ghchinoy/riptide/pkg/results"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/genai"
)

type BenchmarkConfigItem struct {
	Model          string
	ThinkingLevel  string
	ThinkingBudget int32
}

type BenchmarkRunResult struct {
	Item           BenchmarkConfigItem
	SessionID      string
	Outcome        results.Outcome
	Turns          int
	WallSeconds    float64
	ModelSeconds   float64
	HarnessSeconds float64
	AvgTurnLatency float64
	PromptTokens   int
	ThoughtTokens  int
	TotalTokens    int
	ErrorMessage   string
}

var benchCmd = &cobra.Command{
	GroupID: "agent",
	Use:     "bench",
	Short:   "Run benchmark matrix across models and thinking configurations",
	Long: `Bench executes a suite of test runs across Gemini model variants and thinking configurations,
aggregating latency, token usage, and turn metrics to produce a benchmark report.

Results are formatted as Markdown and saved to --output (default: docs/validation/model_benchmarks.md).`,
	Example: `  # Dry-run matrix preview
  riptide bench --dry-run

  # Run standard matrix
  RIPTIDE_NO_TUI=1 riptide bench --max-turns 5

  # Run specific model and thinking levels
  riptide bench --models "gemini-3.5-flash" --thinking-levels "MINIMAL,LOW,HIGH"`,
	RunE: runBench,
}

func init() {
	f := benchCmd.Flags()

	f.String("models", "gemini-3.6-flash,gemini-3.5-flash,gemini-3.5-flash-lite", "Comma-separated model names")
	f.String("thinking-levels", "MINIMAL,LOW,MEDIUM,HIGH", "Comma-separated thinking levels (MINIMAL,LOW,MEDIUM,HIGH)")
	f.String("thinking-budgets", "", "Comma-separated token budgets (e.g. 0,2048,8192)")
	f.String("prompt", "Go to https://www.lotr.com/games/maze and start the game by focusing the canvas and pressing enter", "Benchmark task prompt")
	f.Int("max-turns", 5, "Maximum turns per benchmark run")
	f.String("output", "docs/validation/model_benchmarks.md", "Output file for benchmark report")
	f.Bool("dry-run", false, "Preview execution matrix without running sessions")
	f.String("attach", "", "CDP endpoint to attach to")
	f.String("tab-id", "", "Tab ID for remote browser")
	f.String("tab-url-match", "", "Tab URL match pattern")

	rootCmd.AddCommand(benchCmd)
}

func runBench(cmd *cobra.Command, _ []string) error {
	flags := cmd.Flags()

	modelsStr, _ := flags.GetString("models")
	thinkingLevelsStr, _ := flags.GetString("thinking-levels")
	thinkingBudgetsStr, _ := flags.GetString("thinking-budgets")
	prompt, _ := flags.GetString("prompt")
	maxTurns, _ := flags.GetInt("max-turns")
	outputPath, _ := flags.GetString("output")
	dryRun, _ := flags.GetBool("dry-run")
	attachURL, _ := flags.GetString("attach")
	tabID, _ := flags.GetString("tab-id")
	tabURLMatch, _ := flags.GetString("tab-url-match")

	modelsList := splitAndTrim(modelsStr)
	thinkingLevelsList := splitAndTrim(thinkingLevelsStr)
	thinkingBudgetsList := splitAndTrim(thinkingBudgetsStr)

	// Build matrix items
	var matrix []BenchmarkConfigItem
	for _, m := range modelsList {
		if len(thinkingLevelsList) > 0 {
			for _, lvl := range thinkingLevelsList {
				matrix = append(matrix, BenchmarkConfigItem{
					Model:         m,
					ThinkingLevel: strings.ToUpper(lvl),
				})
			}
		}
		if len(thinkingBudgetsList) > 0 {
			for _, bStr := range thinkingBudgetsList {
				if b, err := strconv.Atoi(bStr); err == nil {
					matrix = append(matrix, BenchmarkConfigItem{
						Model:          m,
						ThinkingBudget: int32(b),
					})
				}
			}
		}
	}

	if len(matrix) == 0 {
		return fmt.Errorf("empty benchmark matrix; specify at least one model and thinking level/budget")
	}

	if dryRun {
		fmt.Printf("Benchmark Execution Matrix (%d runs planned):\n", len(matrix))
		fmt.Printf("%-5s | %-20s | %-15s | %-15s\n", "#", "Model", "Thinking Level", "Thinking Budget")
		fmt.Println(strings.Repeat("-", 62))
		for i, item := range matrix {
			budgetStr := "-"
			if item.ThinkingBudget > 0 {
				budgetStr = fmt.Sprintf("%d tokens", item.ThinkingBudget)
			}
			levelStr := "-"
			if item.ThinkingLevel != "" {
				levelStr = item.ThinkingLevel
			}
			fmt.Printf("%-5d | %-20s | %-15s | %-15s\n", i+1, item.Model, levelStr, budgetStr)
		}
		return nil
	}

	// Validate credentials
	project := viper.GetString("google.project")
	location := viper.GetString("google.location")
	if project == "" || location == "" {
		credentialsHint()
		return fmt.Errorf("missing credentials")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return fmt.Errorf("failed to create GenAI client: %w", err)
	}

	sessionsDir := viper.GetString("sessions.dir")
	if sessionsDir == "" {
		sessionsDir = "sessions"
	}

	fmt.Printf("Starting Riptide Benchmark Matrix (%d runs)...\n", len(matrix))
	fmt.Printf("Prompt: %q (Max turns: %d)\n\n", prompt, maxTurns)

	var runResults []BenchmarkRunResult

	for i, item := range matrix {
		sessionID := uuid.New().String()
		cfgDesc := item.ThinkingLevel
		if cfgDesc == "" {
			cfgDesc = fmt.Sprintf("Budget=%d", item.ThinkingBudget)
		}

		fmt.Printf("[%d/%d] Model: %s | Thinking: %s | Session: %s\n",
			i+1, len(matrix), item.Model, cfgDesc, styleID.Render(sessionID))

		sessionPath := filepath.Join(sessionsDir, sessionID)
		_ = os.MkdirAll(sessionPath, 0755)

		logFile, err := os.OpenFile(
			filepath.Join(sessionPath, "session.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666,
		)
		if err == nil {
			log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		}

		opts := computer.RunOptions{
			MaxTurns:       maxTurns,
			MaxScreenshots: 6,
			ModelName:      item.Model,
			ThinkingLevel:  item.ThinkingLevel,
			ThinkingBudget: item.ThinkingBudget,
			AttachURL:      attachURL,
			TabID:          tabID,
			TabURLMatch:    tabURLMatch,
		}

		// Bench safety handler auto-approves actions
		autoSafetyHandler := func(explanation string) bool {
			return true
		}

		runErr := computer.Run(ctx, client, sessionsDir, sessionID, prompt, nil, autoSafetyHandler, opts)
		if logFile != nil {
			_ = logFile.Close()
		}

		// Parse results from session log
		parsed, err := results.ParseSessionLog(filepath.Join(sessionPath, "session.log"))
		runRes := BenchmarkRunResult{
			Item:      item,
			SessionID: sessionID,
		}

		if runErr != nil {
			runRes.ErrorMessage = runErr.Error()
		}

		if parsed != nil {
			runRes.Outcome = parsed.Outcome
			runRes.Turns = parsed.Turns
			runRes.WallSeconds = parsed.WallSeconds
			runRes.ModelSeconds = parsed.ModelSeconds
			runRes.HarnessSeconds = parsed.HarnessSeconds
			runRes.PromptTokens = parsed.PromptTokens
			runRes.ThoughtTokens = parsed.ThoughtTokens
			runRes.TotalTokens = parsed.TotalTokens

			if parsed.Turns > 0 {
				runRes.AvgTurnLatency = parsed.WallSeconds / float64(parsed.Turns)
			}
		}

		runResults = append(runResults, runRes)
		fmt.Printf("      Outcome: %s | Wall: %.2fs | Model: %.2fs | ThoughtTokens: %d\n\n",
			runRes.Outcome, runRes.WallSeconds, runRes.ModelSeconds, runRes.ThoughtTokens)
	}

	// Generate Report
	reportMD := generateBenchmarkReport(prompt, maxTurns, runResults)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err == nil {
		if err := os.WriteFile(outputPath, []byte(reportMD), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write benchmark report to %s: %v\n", outputPath, err)
		} else {
			fmt.Printf("Benchmark report saved to: %s\n", styleCommand.Render(outputPath))
		}
	}

	fmt.Println("\n" + reportMD)
	return nil
}

func splitAndTrim(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func generateBenchmarkReport(prompt string, maxTurns int, runResults []BenchmarkRunResult) string {
	var sb strings.Builder

	sb.WriteString("# Riptide Model & Thinking Config Benchmark Report\n\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s  \n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Task Prompt:** %q  \n", prompt))
	sb.WriteString(fmt.Sprintf("**Max Turns:** %d  \n\n", maxTurns))

	sb.WriteString("## Matrix Summary\n\n")
	sb.WriteString("| Model | Thinking Config | Outcome | Turns | Wall (s) | Model (s) | Avg Turn (s) | Thought Tokens | Total Tokens |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|\n")

	for _, r := range runResults {
		cfg := r.Item.ThinkingLevel
		if cfg == "" {
			cfg = fmt.Sprintf("Budget=%d", r.Item.ThinkingBudget)
		}

		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %d | %.2f | %.2f | %.2f | %d | %d |\n",
			r.Item.Model, cfg, r.Outcome, r.Turns, r.WallSeconds, r.ModelSeconds, r.AvgTurnLatency, r.ThoughtTokens, r.TotalTokens))
	}

	sb.WriteString("\n## Model Averages\n\n")
	type modelStats struct {
		runs        int
		wallSec     float64
		modelSec    float64
		thoughtTok  int
		totalTok    int
		turns       int
	}
	statsByModel := make(map[string]*modelStats)

	for _, r := range runResults {
		st, ok := statsByModel[r.Item.Model]
		if !ok {
			st = &modelStats{}
			statsByModel[r.Item.Model] = st
		}
		st.runs++
		st.wallSec += r.WallSeconds
		st.modelSec += r.ModelSeconds
		st.thoughtTok += r.ThoughtTokens
		st.totalTok += r.TotalTokens
		st.turns += r.Turns
	}

	sb.WriteString("| Model | Runs | Avg Wall (s) | Avg Model (s) | Avg Turn Latency (s) | Avg Thought Tokens | Avg Total Tokens |\n")
	sb.WriteString("|---|---|---|---|---|---|---|\n")
	for m, st := range statsByModel {
		if st.runs == 0 {
			continue
		}
		avgTurn := 0.0
		if st.turns > 0 {
			avgTurn = st.wallSec / float64(st.turns)
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %d | %.2f | %.2f | %.2f | %d | %d |\n",
			m, st.runs, st.wallSec/float64(st.runs), st.modelSec/float64(st.runs), avgTurn, st.thoughtTok/st.runs, st.totalTok/st.runs))
	}

	sb.WriteString("\n## Recommendations\n\n")
	sb.WriteString("- **Latency-Sensitive Real-Time / Step-Locked Tasks:** Use `gemini-3.5-flash-lite` or `gemini-3.5-flash` with `ThinkingLevel=LOW` or `ThinkingBudget=2048` to minimize turn latency (~2–4s/turn).\n")
	sb.WriteString("- **Complex Multi-Step Reasoning / Audit Tasks:** Use `gemini-3.6-flash` or `gemini-3.5-flash` with `ThinkingLevel=HIGH` or `ThinkingBudget=8192` to maximize solution accuracy.\n")

	return sb.String()
}
