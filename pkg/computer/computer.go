package computer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
	"google.golang.org/genai"
)

const ModelName = "gemini-2.5-computer-use-preview-10-2025"

func Run(ctx context.Context, client *genai.Client, sessionID, prompt string, makeGif bool, observer Observer, safetyHandler SafetyHandler, maxTurns int) error {
	// Helper to emit events
	emit := func(t EventType, msg string, data interface{}) {
		if observer != nil {
			observer(Event{
				Type:      t,
				Message:   msg,
				Data:      data,
				Timestamp: time.Now().Unix(),
			})
		} else {
			// Fallback to log if no observer
			if t == EventError {
				log.Printf("[ERROR] %s: %v", msg, data)
			} else {
				log.Printf("[%s] %s", t, msg)
			}
		}
	}

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
	emit(EventStatus, "Starting Computer Use Session", nil)
	emit(EventLog, fmt.Sprintf("Prompt: %s", prompt), nil)
	
	log.Println("Taking initial screenshot...")
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}
	
	filename := filepath.Join(outputDir, "initial.png")
	if err := os.WriteFile(filename, buf, 0644); err != nil {
		emit(EventLog, fmt.Sprintf("Warning: failed to save screenshot: %v", err), nil)
	}

	history[0].Parts = append(history[0].Parts, &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "image/png",
			Data:     buf,
		},
	})

	defer func() {
		if makeGif {
			emit(EventStatus, "Generating GIF...", nil)
			gifPath := filepath.Join(outputDir, "session.gif")
			cmd := exec.Command("ffmpeg", 
				"-framerate", "1",
				"-i", filepath.Join(outputDir, "turn_%d_post.png"),
				"-y",
				gifPath,
			)
			// Capture output for logging if needed, or silence it
			// cmd.Stdout = os.Stdout 
			// cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				emit(EventError, "Error generating GIF", err)
			} else {
				emit(EventStatus, fmt.Sprintf("GIF generated: %s", gifPath), nil)
			}
		}
	}()

	for i := 0; i < maxTurns; i++ {
				emit(EventStatus, fmt.Sprintf("Turn %d/%d: Sending request...", i+1, maxTurns), nil)
		
				// Debug: Dump history compactly (Disabled for cleaner output, enable for deep debugging)
				/*
				if historyJSON, err := json.Marshal(history); err == nil {
					// Truncate large base64 strings
					// Compact JSON looks like "data":"..."
					re := regexp.MustCompile(`"data":"[^"]{50,}"`)
					truncated := re.ReplaceAllString(string(historyJSON), `"data":"<truncated>"`)
					emit(EventLog, fmt.Sprintf("History: %s", truncated), nil)
				}
				*/
		
				resp, err := client.Models.GenerateContent(ctx, ModelName, history, config)
		if err != nil {
			return fmt.Errorf("generate content failed: %w", err)
		}

		if len(resp.Candidates) == 0 {
			emit(EventLog, "No candidates returned.", nil)
			break
		}
		cand := resp.Candidates[0]
		
		// Add model response to history
		history = append(history, cand.Content)

		hasToolCalls := false
		
		// First pass: Emit thoughts
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				emit(EventThinking, part.Text, nil)
			}
		}

		for _, part := range cand.Content.Parts {
			if part.FunctionCall != nil {
				hasToolCalls = true
				emit(EventAction, fmt.Sprintf("Tool Call: %s", part.FunctionCall.Name), part.FunctionCall.Args)
				
				resultMap, err := Execute(ctx, part.FunctionCall, 1024, 768)
				
				// Handle Safety Interaction if present
				// Executor returns "safety_acknowledgement" = true if it was in the args.
				// But we want to gate it with the handler.
				// Actually, we should check args here BEFORE Execute or inside Execute.
				// Let's do it here or let Execute handle it but we need to inject the handler.
				// For now, let's look at the result.
				// Wait, if we want to PAUSE, we should do it before Execute if possible, 
				// OR Execute needs the handler.
				
				// Let's modify Execute to take the handler? 
				// Or check args here.
				if safety, ok := part.FunctionCall.Args["safety_decision"].(map[string]interface{}); ok {
					explanation, _ := safety["explanation"].(string)
					emit(EventSafety, "Safety Decision Required", explanation)
					
					approved := true
					if safetyHandler != nil {
						approved = safetyHandler(explanation)
					}
					
					if !approved {
						emit(EventStatus, "User denied safety request. Terminating.", nil)
						return nil // Terminate loop
					}
					emit(EventStatus, "Safety request approved. Proceeding.", nil)
				}

				// Capture NEW screenshot for the next state
				var newBuf []byte
				if err := chromedp.Run(ctx, chromedp.Sleep(1*time.Second), chromedp.CaptureScreenshot(&newBuf)); err != nil {
					emit(EventError, "Failed to capture post-action screenshot", err)
				}
				
				// Save post-action screenshot
				postFilename := filepath.Join(outputDir, fmt.Sprintf("turn_%d_post.png", i+1))
				if err := os.WriteFile(postFilename, newBuf, 0644); err != nil {
					emit(EventLog, fmt.Sprintf("Warning: failed to save post screenshot: %v", err), nil)
				} else {
					emit(EventStatus, fmt.Sprintf("Screenshot saved: %s", postFilename), nil)
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
			// If no tool calls, we already emitted thoughts above.
			// Just break the loop.
			break
		}
	}

	return nil
}
