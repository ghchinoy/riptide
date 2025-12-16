package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ghchinoy/website-assistant/pkg/computer"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

func main() {
	prompt := flag.String("prompt", "Go to https://www.google.com and tell me what the doodle is today.", "The prompt for the computer user.")
	makeGif := flag.Bool("gif", false, "Generate a GIF of the session.")
	flag.Parse()

	ctx := context.Background()
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

	if err := computer.Run(ctx, client, sessionID, *prompt, *makeGif); err != nil {
		log.Fatalf("Computer Use failed: %v", err)
	}
}
