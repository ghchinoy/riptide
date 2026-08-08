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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ghchinoy/riptide/pkg/computer"
	"github.com/ghchinoy/riptide/pkg/tui"
	"github.com/ghchinoy/riptide/pkg/viewer"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/genai"
)

var runCmd = &cobra.Command{
	GroupID: "agent",
	Use:     "run",
	Short:   "Run an agent session against a URL or current browser state",
	Long: `Run starts a Gemini 3.5 Flash agent harness session.

The agent captures a screenshot, sends it to Gemini with your prompt, and
executes the returned actions (clicks, typing, scrolling) in a headless Chrome
browser. It loops until the goal is achieved, the turn limit is reached, or
you press Ctrl+C.

Credentials are loaded from (in priority order):
  1. GOOGLE_CLOUD_PROJECT / GOOGLE_CLOUD_LOCATION env vars
  2. ~/.config/riptide/config.yaml  (run 'riptide config init' to create)
  3. Local .env file`,
	Example: `  # Basic session with TUI
  riptide run --prompt "Go to google.com and search for 'Gemini 3.5'"

  # Headless / agent-friendly mode
  RIPTIDE_NO_TUI=1 riptide run --prompt "..." --max-turns 20

  # Visual audit mode
  riptide run --prompt "Check the homepage" --mode audit

  # Show the browser window for debugging
  riptide run --prompt "..." --show-browser

  # Serve the session viewer alongside the run
  riptide run --prompt "..." --serve

  # Override the model for this run only
  riptide run --prompt "..." --model gemini-3.5-flash`,
	RunE: runSession,
}

func init() {
	f := runCmd.Flags()

	f.StringP("prompt", "p", "", "The task for the agent to complete (required)")
	f.Int("max-turns", 0, "Maximum interaction turns (0 = use config/default)")
	f.Int("max-screenshots", 0, "Screenshots to keep in context (0 = use config/default)")
	f.Bool("gif", false, "Generate a session.gif replay")
	f.Bool("show-browser", false, "Show the Chrome window (disable headless)")
	f.Bool("axt", false, "Inject Accessibility Tree alongside screenshots")
	f.String("mode", "", "Operation mode: default | audit")
	f.Bool("tui", false, "Use the interactive TUI (overrides tui.enabled config)")
	f.Bool("quit-on-exit", false, "Auto-exit TUI when session finishes")
	f.Bool("high-contrast", false, "High-contrast TUI theme")
	f.String("user-agent", "", "Custom browser User-Agent string")
	f.Bool("transparent-ua", false, "Append Riptide identifier to User-Agent")
	f.Bool("serve", false, "Start session viewer alongside the agent")
	// hyt.1: full-page debug screenshots default OFF (expensive resize-capture-resize cycle).
	f.Bool("debug-screenshots", false, "Save full-page turn_N_full.png screenshots (slow on tall SPAs, useful for debugging)")
	f.String("model", "", "Gemini model to use for computer use (empty = use config/model.name, default: "+computer.DefaultModelName+")")
	f.Int("thinking-budget", 0, "Token budget for internal reasoning (0 = use config/model.thinking_budget)")
	f.String("thinking-level", "", "Thinking level for internal reasoning (MINIMAL | LOW | MEDIUM | HIGH)")
	f.String("attach", "", "CDP endpoint of an already-running Chrome instance to attach to (e.g. http://localhost:9222)")
	f.String("tab-id", "", "Target ID of a specific open tab to attach to (requires --attach)")
	f.String("tab-url-match", "", "Substring match to pick an open tab by URL (requires --attach)")

	_ = runCmd.MarkFlagRequired("prompt")
}

func runSession(cmd *cobra.Command, _ []string) error {
	// Validate credentials before doing any heavy work.
	project := viper.GetString("google.project")
	location := viper.GetString("google.location")
	if project == "" || location == "" {
		credentialsHint()
		return fmt.Errorf("missing credentials")
	}

	// Resolve all settings: flag > viper (env/file/default).
	flags := cmd.Flags()
	resolveBool := func(flag, key string) bool {
		if flags.Changed(flag) {
			v, _ := flags.GetBool(flag)
			return v
		}
		return viper.GetBool(key)
	}
	resolveInt := func(flag, key string) int {
		if flags.Changed(flag) {
			v, _ := flags.GetInt(flag)
			return v
		}
		return viper.GetInt(key)
	}

	resolveString := func(flag, key string) string {
		if flags.Changed(flag) {
			v, _ := flags.GetString(flag)
			return v
		}
		if key != "" && viper.IsSet(key) {
			return viper.GetString(key)
		}
		return ""
	}

	prompt, _ := flags.GetString("prompt")
	maxTurns := resolveInt("max-turns", "session.max_turns")
	maxScreenshots := resolveInt("max-screenshots", "session.max_screenshots")
	makeGif := resolveBool("gif", "session.gif")
	showBrowser := resolveBool("show-browser", "session.show_browser")
	useAXT := resolveBool("axt", "session.axt")
	mode := resolveString("mode", "session.mode")
	useTUI := resolveBool("tui", "tui.enabled")
	quitOnExit := resolveBool("quit-on-exit", "tui.quit_on_exit")
	highContrast := resolveBool("high-contrast", "tui.high_contrast")
	serve := resolveBool("serve", "")
	debugScreenshots := resolveBool("debug-screenshots", "session.debug_screenshots")
	sessionsDir := viper.GetString("sessions.dir")
	modelName := resolveString("model", "model.name")
	if modelName == "" {
		modelName = computer.DefaultModelName
	}
	thinkingBudget := int32(resolveInt("thinking-budget", "model.thinking_budget"))
	thinkingLevel := resolveString("thinking-level", "model.thinking_level")
	attachURL := resolveString("attach", "browser.attach_url")
	tabID := resolveString("tab-id", "browser.tab_id")
	tabURLMatch := resolveString("tab-url-match", "browser.tab_url_match")

	if flags.Changed("thinking-budget") && flags.Changed("thinking-level") {
		return fmt.Errorf("cannot specify both --thinking-budget and --thinking-level; choose one")
	}
	if tabID != "" && tabURLMatch != "" {
		return fmt.Errorf("cannot specify both --tab-id and --tab-url-match; choose one")
	}
	if attachURL == "" && (tabID != "" || tabURLMatch != "") {
		return fmt.Errorf("--tab-id and --tab-url-match require --attach <cdp-url>")
	}
	if attachURL != "" && flags.Changed("show-browser") {
		fmt.Fprintf(os.Stderr, "%s --show-browser is ignored when attaching to an existing browser via --attach\n", styleWarn.Render("Warning:"))
	}

	warnIfLegacyComputerUseModel(modelName)

	// Respect RIPTIDE_NO_TUI for AX/headless mode.
	if os.Getenv("RIPTIDE_NO_TUI") != "" {
		useTUI = false
	}

	// Build User-Agent.
	ua := viper.GetString("session.user_agent")
	if flags.Changed("user-agent") {
		ua, _ = flags.GetString("user-agent")
	}
	transparentUA := resolveBool("transparent-ua", "session.transparent_ua")
	if transparentUA {
		ua = ua + " (Riptide; +https://github.com/ghchinoy/riptide)"
	}

	// Start optional session viewer.
	if serve {
		port := fmt.Sprintf(":%d", viper.GetInt("viewer.port"))
		go func() {
			if err := viewer.Start(port); err != nil {
				log.Printf("Session viewer error: %v", err)
			}
		}()
	}

	// Create GenAI client.
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

	// Set up session directory and logging.
	sessionID := uuid.New().String()
	sessionPath := filepath.Join(sessionsDir, sessionID)
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	logFile, err := os.OpenFile(
		filepath.Join(sessionPath, "session.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666,
	)
	if err != nil {
		return fmt.Errorf("failed to open session log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	if useTUI {
		log.SetOutput(logFile)
	} else {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}

	opts := computer.RunOptions{
		MakeGif:          makeGif,
		ShowBrowser:      showBrowser,
		DebugScreenshots: debugScreenshots,
		UserAgent:        ua,
		UseAXT:           useAXT,
		MaxTurns:         maxTurns,
		MaxScreenshots:   maxScreenshots,
		Mode:             mode,
		ModelName:        modelName,
		ThinkingBudget:   thinkingBudget,
		ThinkingLevel:    thinkingLevel,
		AttachURL:        attachURL,
		TabID:            tabID,
		TabURLMatch:      tabURLMatch,
	}

	if !useTUI {
		return runHeadless(ctx, client, sessionsDir, sessionID, prompt, serve, opts)
	}
	return runTUI(ctx, client, sessionsDir, sessionID, prompt, quitOnExit, highContrast, serve, opts)
}

// knownLegacyComputerUseModels lists model names that require the "legacy"
// computer-use function-call dialect (predefined function names such as
// click_at/hover_at rather than the Gemini 3.5 Flash native names). Riptide's
// Go harness (pkg/computer/tools_standard.go) only dispatches the 3.5-style
// dialect, so runs against these models are not expected to work correctly.
// This is a warning, not a hard block, since model names and capabilities
// change over time and we don't want to reject models we don't yet know about.
var knownLegacyComputerUseModels = []string{
	"gemini-2.5-computer-use-preview-10-2025",
	"gemini-3-flash-preview",
	"gemini-3.1-pro-preview",
}

// warnIfLegacyComputerUseModel prints a stderr warning if modelName is known
// to require the legacy computer-use function-call dialect, which this
// harness does not implement. The run is still allowed to proceed.
func warnIfLegacyComputerUseModel(modelName string) {
	for _, legacy := range knownLegacyComputerUseModels {
		if modelName == legacy {
			fmt.Fprintf(os.Stderr,
				"%s model %q uses the legacy computer-use function-call dialect, which Riptide's harness does not support. Actions may fail or be misinterpreted. Use %q instead.\n",
				styleWarn.Render("Warning:"), modelName, computer.DefaultModelName)
			return
		}
	}
}

func runHeadless(ctx context.Context, client *genai.Client,
	sessionsDir, sessionID, prompt string, serve bool,
	opts computer.RunOptions,
) error {
	fmt.Printf("Starting session: %s\n", styleID.Render(sessionID))

	safetyHandler := func(explanation string) bool {
		fmt.Printf("\n%s %s\n", styleWarn.Render("[SAFETY]"), explanation)
		fmt.Print("Proceed? (y/N): ")
		var r string
		if _, err := fmt.Scanln(&r); err != nil {
			return false
		}
		return r == "y" || r == "Y"
	}

	var observer computer.Observer
	if serve {
		observer = func(e computer.Event) {
			if b, err := json.Marshal(e); err == nil {
				viewer.BroadcastEvent(sessionID, b)
			}
		}
	}

	err := computer.Run(ctx, client, sessionsDir, sessionID, prompt, observer, safetyHandler, opts)
	if err != nil && err != context.Canceled {
		return fmt.Errorf("session failed: %w", err)
	}
	return nil
}

func runTUI(ctx context.Context, client *genai.Client,
	sessionsDir, sessionID, prompt string,
	quitOnExit, highContrast, serve bool,
	opts computer.RunOptions,
) error {
	m := tui.NewModel(sessionsDir, sessionID, quitOnExit, highContrast)
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		tuiObserver := m.GetObserver(p)
		var observer computer.Observer
		if serve {
			observer = func(e computer.Event) {
				tuiObserver(e)
				if b, err := json.Marshal(e); err == nil {
					viewer.BroadcastEvent(sessionID, b)
				}
			}
		} else {
			observer = tuiObserver
		}

		err := computer.Run(ctx, client, sessionsDir, sessionID, prompt, observer, m.GetSafetyHandler(p), opts)
		if err != nil && err != context.Canceled {
			p.Send(computer.Event{Type: computer.EventError,
				Message: fmt.Sprintf("Fatal: %v", err)})
		}
		if quitOnExit {
			time.Sleep(2 * time.Second)
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
