package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"os/signal"
	"syscall"

	"github.com/ghchinoy/website-assistant/pkg/computer"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

func main() {
	prompt := flag.String("prompt", "Go to https://www.google.com and tell me what the doodle is today.", "The prompt for the computer user.")
	makeGif := flag.Bool("gif", false, "Generate a GIF of the session.")
	maxTurns := flag.Int("max-turns", 10, "Maximum number of interaction turns.")
	maxScreenshots := flag.Int("max-screenshots", 3, "Maximum number of recent screenshots to keep in history context.")
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

	// We pass the signal-aware context to Run. 
	// If the user hits Ctrl+C, ctx.Done() will close, and chromedp/genai should terminate gracefully.
	if err := computer.Run(ctx, client, sessionID, *prompt, *makeGif, nil, safetyHandler, *maxTurns, *maxScreenshots); err != nil {
		if err == context.Canceled {
			fmt.Println("\nRun cancelled by user.")
		} else {
			log.Fatalf("Computer Use failed: %v", err)
		}
	}
}
