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
  riptide run --prompt "..." --serve`,
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
	resolve := func(flag, key string) interface{} {
		if flags.Changed(flag) {
			v, _ := flags.GetString(flag)
			return v
		}
		return viper.Get(key)
	}
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

	prompt, _ := flags.GetString("prompt")
	maxTurns := resolveInt("max-turns", "session.max_turns")
	maxScreenshots := resolveInt("max-screenshots", "session.max_screenshots")
	makeGif := resolveBool("gif", "session.gif")
	showBrowser := resolveBool("show-browser", "session.show_browser")
	useAXT := resolveBool("axt", "session.axt")
	mode := resolve("mode", "session.mode").(string)
	useTUI := resolveBool("tui", "tui.enabled")
	quitOnExit := resolveBool("quit-on-exit", "tui.quit_on_exit")
	highContrast := resolveBool("high-contrast", "tui.high_contrast")
	serve := resolveBool("serve", "")
	debugScreenshots := resolveBool("debug-screenshots", "session.debug_screenshots")
	sessionsDir := viper.GetString("sessions.dir")

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

	if !useTUI {
		return runHeadless(ctx, client, sessionsDir, sessionID, prompt,
			makeGif, showBrowser, debugScreenshots, ua, useAXT, maxTurns, maxScreenshots, mode, serve)
	}
	return runTUI(ctx, client, sessionsDir, sessionID, prompt,
		makeGif, showBrowser, debugScreenshots, ua, useAXT, maxTurns, maxScreenshots, mode,
		quitOnExit, highContrast, serve)
}

func runHeadless(ctx context.Context, client *genai.Client,
	sessionsDir, sessionID, prompt string,
	makeGif, showBrowser, debugScreenshots bool, ua string, useAXT bool,
	maxTurns, maxScreenshots int, mode string, serve bool,
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

	err := computer.Run(ctx, client, sessionsDir, sessionID, prompt,
		makeGif, showBrowser, debugScreenshots, ua, useAXT, observer, safetyHandler,
		maxTurns, maxScreenshots, mode)
	if err != nil && err != context.Canceled {
		return fmt.Errorf("session failed: %w", err)
	}
	return nil
}

func runTUI(ctx context.Context, client *genai.Client,
	sessionsDir, sessionID, prompt string,
	makeGif, showBrowser, debugScreenshots bool, ua string, useAXT bool,
	maxTurns, maxScreenshots int, mode string,
	quitOnExit, highContrast, serve bool,
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

		err := computer.Run(ctx, client, sessionsDir, sessionID, prompt,
			makeGif, showBrowser, debugScreenshots, ua, useAXT, observer,
			m.GetSafetyHandler(p), maxTurns, maxScreenshots, mode)
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
