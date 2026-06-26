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
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var sessionsCmd = &cobra.Command{
	GroupID: "viewer",
	Use:     "sessions",
	Short:   "List and inspect agent sessions",
	Long:    `Browse agent sessions stored in the sessions directory.`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent agent sessions",
	Long:  `List agent sessions, newest first. Default limit: 10.`,
	Example: `  riptide sessions list
  riptide sessions list --limit 25
  riptide sessions list --json`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		dir := viper.GetString("sessions.dir")
		limit, _ := cmd.Flags().GetInt("limit")
		useJSON := jsonMode(cmd)

		sessions, err := loadSessions(dir, limit)
		if err != nil {
			return fmt.Errorf("could not read sessions directory %q: %w\n  Hint: run 'riptide run --prompt ...' to create your first session", dir, err)
		}

		if len(sessions) == 0 {
			if useJSON {
				fmt.Println("[]")
				return nil
			}
			fmt.Println(muted("No sessions found in " + dir))
			fmt.Println(muted("  Run 'riptide run --prompt \"...\"' to start one."))
			return nil
		}

		if useJSON {
			return json.NewEncoder(os.Stdout).Encode(sessions)
		}

		fmt.Println()
		fmt.Printf("  %s  %-20s  %-10s  %s\n",
			styleAccent.Render(fmt.Sprintf("%-36s", "SESSION ID")),
			styleAccent.Render("STARTED"),
			styleAccent.Render("STATUS"),
			styleAccent.Render("PROMPT"),
		)
		fmt.Println("  " + separator(100))

		for _, s := range sessions {
			promptPreview := s.Prompt
			if len(promptPreview) > 55 {
				promptPreview = promptPreview[:52] + "..."
			}
			statusStyle := styleMuted
			switch s.Status {
			case "finished":
				statusStyle = stylePass
			case "active":
				statusStyle = styleWarn
			}
			ts := s.Timestamp.Format("2006-01-02 15:04")
			fmt.Printf("  %s  %-20s  %-10s  %s\n",
				styleID.Render(fmt.Sprintf("%-36s", s.ID)),
				muted(ts),
				statusStyle.Render(fmt.Sprintf("%-10s", s.Status)),
				promptPreview,
			)
		}
		fmt.Println()
		return nil
	},
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show the detail of a specific session",
	Long:  `Display the turns, actions, and thinking of a completed agent session.`,
	Example: `  riptide sessions show <uuid>
  riptide sessions show <uuid> --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := viper.GetString("sessions.dir")
		id := args[0]
		useJSON := jsonMode(cmd)

		logPath := filepath.Join(dir, id, "session.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return fmt.Errorf("session %q not found in %s", id, dir)
		}

		s, err := parseSession(id, logPath)
		if err != nil {
			return fmt.Errorf("could not parse session: %w", err)
		}

		if useJSON {
			return json.NewEncoder(os.Stdout).Encode(s)
		}

		// Human-readable output.
		fmt.Println()
		fmt.Printf("%s %s\n", header("Session"), styleID.Render(id))
		fmt.Printf("  %s %s\n", muted("Started:"), s.Timestamp.Format(time.RFC3339))
		fmt.Printf("  %s %s\n", muted("Status: "), func() string {
			if s.Status == "finished" {
				return stylePass.Render(s.Status)
			}
			return styleWarn.Render(s.Status)
		}())
		fmt.Printf("  %s %s\n", muted("Prompt: "), s.Prompt)
		fmt.Println()

		for _, t := range s.Turns {
			fmt.Printf("%s %s\n",
				styleAccent.Render(fmt.Sprintf("Turn %d", t.Index)),
				separator(40),
			)
			for _, thought := range t.Thinking {
				fmt.Printf("  %s %s\n", styleMuted.Render("[think]"), thought)
			}
			if t.Action != "" {
				fmt.Printf("  %s %s\n", styleWarn.Render("[act]  "), t.Action)
			}
			if t.Screenshot != "" {
				fmt.Printf("  %s %s\n", styleMuted.Render("[shot] "),
					filepath.Join(dir, id, t.Screenshot))
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	sessionsListCmd.Flags().Int("limit", 10, "Maximum sessions to show")
	sessionsCmd.AddCommand(sessionsListCmd, sessionsShowCmd)
}

// jsonMode returns true if --json was passed or RIPTIDE_NO_TUI is set.
func jsonMode(cmd *cobra.Command) bool {
	if os.Getenv("RIPTIDE_NO_TUI") != "" {
		return true
	}
	v, _ := cmd.Root().PersistentFlags().GetBool("json")
	return v
}

// sessionRecord is the data model for a parsed session.
type sessionRecord struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Prompt    string      `json:"prompt"`
	Status    string      `json:"status"`
	Turns     []turnRecord `json:"turns,omitempty"`
}

type turnRecord struct {
	Index      int      `json:"index"`
	Thinking   []string `json:"thinking"`
	Action     string   `json:"action"`
	Screenshot string   `json:"screenshot"`
	FullPage   string   `json:"full_page"`
}

// loadSessions reads up to limit sessions from dir, sorted newest-first.
func loadSessions(dir string, limit int) ([]sessionRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []sessionRecord
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		logPath := filepath.Join(dir, e.Name(), "session.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			continue
		}
		info, _ := e.Info()
		prompt, status := peekSessionMetadata(logPath)
		sessions = append(sessions, sessionRecord{
			ID:        e.Name(),
			Timestamp: info.ModTime(),
			Prompt:    prompt,
			Status:    status,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

// parseSession reads a session.log and extracts turns with thinking and actions.
func parseSession(id, logPath string) (*sessionRecord, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, _ := f.Stat()
	s := &sessionRecord{ID: id, Timestamp: info.ModTime(), Status: "active"}

	var turns []turnRecord
	var cur *turnRecord

	turnRe := regexp.MustCompile(`\[status\] Turn (\d+)/`)
	promptRe := regexp.MustCompile(`\[log\] Prompt: (.+)`)
	thinkRe := regexp.MustCompile(`\[thinking\] (.+)`)
	actionRe := regexp.MustCompile(`\[action\] Tool Call: (.+)`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if m := promptRe.FindStringSubmatch(line); len(m) > 1 {
			s.Prompt = strings.TrimSuffix(m[1], " <nil>")
		}
		if strings.ContainsAny(line, "Session Finished.Max Turns Reached.Goal Achieved.Fatal:") {
			s.Status = "finished"
		}
		if m := turnRe.FindStringSubmatch(line); len(m) > 1 {
			if cur != nil {
				turns = append(turns, *cur)
			}
			var idx int
			fmt.Sscanf(m[1], "%d", &idx)
			cur = &turnRecord{
				Index:      idx,
				Thinking:   []string{},
				Screenshot: fmt.Sprintf("screenshots/turn_%d_post.png", idx),
				FullPage:   fmt.Sprintf("screenshots/turn_%d_full.png", idx),
			}
		}
		if cur != nil {
			if m := thinkRe.FindStringSubmatch(line); len(m) > 1 {
				cur.Thinking = append(cur.Thinking, strings.TrimSuffix(m[1], " <nil>"))
			}
			if m := actionRe.FindStringSubmatch(line); len(m) > 1 {
				cur.Action = strings.TrimSuffix(m[1], " <nil>")
			}
		}
	}
	if cur != nil {
		turns = append(turns, *cur)
	}
	s.Turns = turns
	return s, scanner.Err()
}

// peekSessionMetadata quickly reads a log file to find the prompt and final status.
func peekSessionMetadata(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", "unknown"
	}
	defer func() { _ = f.Close() }()

	prompt, status := "", "active"
	re := regexp.MustCompile(`\[log\] Prompt: (.+)`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			prompt = strings.TrimSuffix(m[1], " <nil>")
		}
		if strings.Contains(line, "Session Finished.") ||
			strings.Contains(line, "Max Turns Reached.") ||
			strings.Contains(line, "Goal Achieved.") ||
			strings.Contains(line, "Fatal:") {
			status = "finished"
		}
	}
	return prompt, status
}
