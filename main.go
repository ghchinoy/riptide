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

package main

import (
        "context"
        "flag"
        "fmt"
        "log"
        "os"
        "path/filepath"
        "time"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ghchinoy/riptide/pkg/computer"
	"github.com/ghchinoy/riptide/pkg/tui"
	"github.com/ghchinoy/riptide/pkg/utils"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

func main() {

        // Load environment variables from .env or XDG config

        utils.LoadConfig()



        prompt := flag.String("prompt", "", "The prompt for the computer user. (Mandatory)")


	makeGif := flag.Bool("gif", false, "Generate a GIF of the session.")
	maxTurns := flag.Int("max-turns", 10, "Maximum number of interaction turns.")
	maxScreenshots := flag.Int("max-screenshots", 3, "Maximum number of recent screenshots to keep in history context.")
	        useTUI := flag.Bool("tui", true, "Use the Bubble Tea TUI.")
	        quitOnExit := flag.Bool("quit-on-exit", false, "Automatically exit the TUI when the session finishes.")
	        mode := flag.String("mode", "default", "The mode of operation (default, audit).")
	        showBrowser := flag.Bool("show-browser", false, "Show the browser window (disable headless mode).")
	        sessionsDir := flag.String("sessions-dir", "sessions", "Directory to store session logs and screenshots.")
	        flag.Parse()
	
	        if *prompt == "" {
	                fmt.Println("Error: The -prompt flag is mandatory.")
	                flag.Usage()
	                os.Exit(1)
	        }
	        // Handle Ctrl+C
	        ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	        defer cancel()
	
	        projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	        location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	
	        if projectId == "" || location == "" {
	                log.Fatal("GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION must be set")
	        }
	
	        client, err := genai.NewClient(ctx, &genai.ClientConfig{
	                Project:  projectId,
	                Location: location,
	                Backend:  genai.BackendVertexAI,
	        })
	        if err != nil {
	                log.Fatalf("Failed to create GenAI client: %v", err)
	        }
	
	        sessionID := uuid.New().String()
	        sessionPath := filepath.Join(*sessionsDir, sessionID)
	
	        if *useTUI {
	                // Create session directory
	                if err := os.MkdirAll(sessionPath, 0755); err != nil {
	                        log.Fatalf("Failed to create session directory: %v", err)
	                }
	                // Redirect log output to a file for this session
	                logFile, err := os.OpenFile(filepath.Join(sessionPath, "session.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	                if err == nil {
	                        log.SetOutput(logFile)
	                        defer logFile.Close()
	                }
	        }
	
	        if !*useTUI {
	                fmt.Printf("Starting Session: %s\n", sessionID)
	                // Interactive Safety Handler
	                safetyHandler := func(explanation string) bool {
	                        fmt.Printf("\n[SAFETY ALERT] The model flagged a safety concern:\n%s\n", explanation)
	                        fmt.Print("Do you want to proceed? (y/N): ")
	                        var response string
	                        _, err := fmt.Scanln(&response)
	                        if err != nil {
	                                return false // Assume no on error/EOF
	                        }
	                        return response == "y" || response == "Y" || response == "yes"
	                }
	
	                if err := computer.Run(ctx, client, *sessionsDir, sessionID, *prompt, *makeGif, *showBrowser, nil, safetyHandler, *maxTurns, *maxScreenshots, *mode); err != nil {
	                        if err == context.Canceled {
	                                fmt.Println("\nRun cancelled by user.")
	                        } else {
	                                log.Fatalf("Computer Use failed: %v", err)
	                        }
	                }
	                return
	        }
	
	        // TUI Mode
	        m := tui.NewModel(*sessionsDir, sessionID, *quitOnExit)
	        p := tea.NewProgram(m, tea.WithAltScreen())
	
	        // Run agent in goroutine
	        go func() {
	                observer := m.GetObserver(p)
	                safetyHandler := m.GetSafetyHandler(p)
	
	                err := computer.Run(ctx, client, *sessionsDir, sessionID, *prompt, *makeGif, *showBrowser, observer, safetyHandler, *maxTurns, *maxScreenshots, *mode)
	                if err != nil {
	                        if err != context.Canceled {
	                                p.Send(computer.Event{
	                                        Type:    computer.EventError,
	                                        Message: fmt.Sprintf("Fatal: %v", err),
	                                })
	                        }
	                }
	
	                if *quitOnExit {
	                        time.Sleep(2 * time.Second)
	                }
	        }()
	
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI Error: %v", err)
	}
}
