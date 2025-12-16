package computer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/chromedp/chromedp"
	"google.golang.org/genai"
)

const ModelName = "gemini-2.5-computer-use-preview-10-2025"

func Run(ctx context.Context, client *genai.Client, sessionID, prompt string, makeGif bool) error {
	// 0. Setup Output
	outputDir := filepath.Join("screenshots", sessionID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// 1. Setup Chromedp
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1024, 768),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// 2. Configure Model Config
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				ComputerUse: &genai.ComputerUse{},
			},
		},
	}

	// 3. Main Loop
	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		return fmt.Errorf("failed to navigate to blank: %w", err)
	}

	var history []*genai.Content

	// Initial user message
	history = append(history, &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: prompt},
		},
	})

	// Capture initial screenshot
	log.Println("Taking initial screenshot...")
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}
	
	filename := filepath.Join(outputDir, "initial.png")
	if err := os.WriteFile(filename, buf, 0644); err != nil {
		log.Printf("Warning: failed to save screenshot: %v", err)
	}

	history[0].Parts = append(history[0].Parts, &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "image/png",
			Data:     buf,
		},
	})

	defer func() {
		if makeGif {
			log.Println("Generating GIF...")
			gifPath := filepath.Join(outputDir, "session.gif")
			cmd := exec.Command("ffmpeg", 
				"-framerate", "1",
				"-i", filepath.Join(outputDir, "turn_%d_post.png"),
				"-y",
				gifPath,
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Printf("Error generating GIF: %v", err)
			} else {
				log.Printf("GIF generated: %s", gifPath)
			}
		}
	}()

	for i := 0; i < 10; i++ {
		log.Printf("Turn %d: Sending request...", i+1)

		// Debug: Dump history with truncation
		if historyJSON, err := json.MarshalIndent(history, "", "  "); err == nil {
			// Truncate large base64 strings for logging
			// Look for "data": "..."
			re := regexp.MustCompile(`"data":\s*"[^"]{50,}"`)
			truncated := re.ReplaceAllString(string(historyJSON), `"data": "<truncated_base64_data>"`)
			log.Printf("Request History:\n%s", truncated)
		}

		resp, err := client.Models.GenerateContent(ctx, ModelName, history, config)
		if err != nil {
			return fmt.Errorf("generate content failed: %w", err)
		}

		if len(resp.Candidates) == 0 {
			log.Println("No candidates returned.")
			break
		}
		cand := resp.Candidates[0]
		
		// Add model response to history
		history = append(history, cand.Content)

		hasToolCalls := false
		for _, part := range cand.Content.Parts {
			if part.FunctionCall != nil {
				hasToolCalls = true
				log.Printf("Tool Call: %s", part.FunctionCall.Name)
				
				resultMap, err := Execute(ctx, part.FunctionCall, 1024, 768)
				
				// Capture NEW screenshot for the next state
				var newBuf []byte
				if err := chromedp.Run(ctx, chromedp.Sleep(1*time.Second), chromedp.CaptureScreenshot(&newBuf)); err != nil {
					log.Printf("Failed to capture post-action screenshot: %v", err)
				}
				
				// Save post-action screenshot
				postFilename := filepath.Join(outputDir, fmt.Sprintf("turn_%d_post.png", i+1))
				if err := os.WriteFile(postFilename, newBuf, 0644); err != nil {
					log.Printf("Warning: failed to save post screenshot: %v", err)
				}

				toolResp := &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name: part.FunctionCall.Name,
						Response: resultMap,
						Parts: []*genai.FunctionResponsePart{
							{
								InlineData: &genai.FunctionResponseBlob{
									MIMEType: "image/png",
									Data:     newBuf,
								},
							},
						},
					},
				}
				if err != nil {
					if toolResp.FunctionResponse.Response == nil {
						toolResp.FunctionResponse.Response = make(map[string]interface{})
					}
					toolResp.FunctionResponse.Response["error"] = err.Error()
				}

				history = append(history, &genai.Content{
					Role: "user",
					Parts: []*genai.Part{
						toolResp,
					},
				})
			}
		}

		if !hasToolCalls {
			for _, part := range cand.Content.Parts {
				if part.Text != "" {
					fmt.Printf("Model: %s\n", part.Text)
				}
			}
			break
		}
	}

	return nil
}
