package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ghchinoy/website-assistant/pkg/computer"
	"github.com/ghchinoy/website-assistant/pkg/tui"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

func main() {
	prompt := flag.String("prompt", "Go to https://www.google.com and tell me what the doodle is today.", "The prompt for the computer user.")
	makeGif := flag.Bool("gif", false, "Generate a GIF of the session.")
	maxTurns := flag.Int("max-turns", 10, "Maximum number of interaction turns.")
	maxScreenshots := flag.Int("max-screenshots", 3, "Maximum number of recent screenshots to keep in history context.")
	useTUI := flag.Bool("tui", true, "Use the Bubble Tea TUI.")
	autoExit := flag.Bool("auto-exit", false, "Automatically exit the TUI when the session finishes.")
	mode := flag.String("mode", "default", "The mode of operation (default, audit).")
	flag.Parse()

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

	if *useTUI {
		// Create logs directory if it doesn't exist
		if err := os.MkdirAll("logs", 0755); err != nil {
			log.Fatalf("Failed to create logs directory: %v", err)
		}
		// Redirect log output to a file for this session
		logFile, err := os.OpenFile(fmt.Sprintf("logs/session_%s.log", sessionID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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

		if err := computer.Run(ctx, client, sessionID, *prompt, *makeGif, nil, safetyHandler, *maxTurns, *maxScreenshots, *mode); err != nil {
			if err == context.Canceled {
				fmt.Println("\nRun cancelled by user.")
			} else {
				log.Fatalf("Computer Use failed: %v", err)
			}
		}
		return
	}

	// TUI Mode
	m := tui.NewModel(sessionID, *autoExit)
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run agent in goroutine
	go func() {
		observer := m.GetObserver(p)
		safetyHandler := m.GetSafetyHandler(p)

		err := computer.Run(ctx, client, sessionID, *prompt, *makeGif, observer, safetyHandler, *maxTurns, *maxScreenshots, *mode)
		if err != nil {
			if err != context.Canceled {
				p.Send(computer.Event{
					Type:    computer.EventError,
					Message: fmt.Sprintf("Fatal: %v", err),
				})
			}
		}

		if *autoExit {
			time.Sleep(2 * time.Second)
		}
	}()

	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI Error: %v", err)
	}
}
