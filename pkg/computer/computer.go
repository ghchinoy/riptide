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

func Run(ctx context.Context, client *genai.Client, sessionID, prompt string, makeGif bool, observer Observer, safetyHandler SafetyHandler, maxTurns, maxScreenshots int, mode string) error {
	// Helper to emit events
	emit := func(t EventType, msg string, data interface{}) {
		// Always log to session log file as well
		log.Printf("[%s] %s %+v", t, msg, data)
		if observer != nil {
			observer(Event{
				Type:      t,
				Message:   msg,
				Data:      data,
				Timestamp: time.Now().Unix(),
			})
		}
	}

	// 0. Setup Output
	outputDir := filepath.Join("screenshots", sessionID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// 1. Setup Chromedp
	emit(EventStatus, "Initializing browser allocator...", nil)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1024, 768),
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.DisableGPU,
	)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	emit(EventStatus, "Creating browser context...", nil)
	ctx, cancel = chromedp.NewContext(allocCtx, 
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// Listen for console logs


	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{
				ComputerUse: &genai.ComputerUse{},
			},
		},
		SafetySettings: []*genai.SafetySetting{
			{
				Category:  genai.HarmCategoryHateSpeech,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
			{
				Category:  genai.HarmCategoryDangerousContent,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
			{
				Category:  genai.HarmCategoryHarassment,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
			{
				Category:  genai.HarmCategorySexuallyExplicit,
				Threshold: genai.HarmBlockThresholdBlockNone,
			},
		},
	}

	// 3. Pre-flight Diagnostics
	emit(EventStatus, "Running pre-flight diagnostics...", nil)
	// Create a sub-context just for pre-flight
	pfCtx, pfCancel := context.WithTimeout(ctx, 30*time.Second)
	
	if err := chromedp.Run(pfCtx, chromedp.Navigate("about:blank")); err != nil {
		pfCancel()
		return fmt.Errorf("ENVIRONMENT NOT READY: failed to launch browser or navigate to blank page: %w. Please ensure Chrome/Chromium is installed and accessible.", err)
	}
	var finalURL string
	if err := chromedp.Run(pfCtx, chromedp.Location(&finalURL)); err == nil {
		log.Printf("Pre-flight final URL: %s", finalURL)
	}
	pfCancel()
	emit(EventStatus, "Environment ready.", nil)

	var history []*genai.Content

	// Initial user message
	fullPrompt := prompt
	if mode == "audit" {
		fullPrompt += "\n\nAdditionally, perform a 'Visual Health' audit of the page. Identify contrast violations, elements overflowing containers, or inconsistent margins. Return your final report in a structured JSON format if possible."
	}

	history = append(history, &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: fullPrompt},
		},
	})

	// Capture initial screenshot
	emit(EventStatus, "Starting Computer Use Session", nil)
	emit(EventLog, fmt.Sprintf("Prompt: %s", prompt), nil)
	
	log.Println("Taking initial screenshot...")
	time.Sleep(1 * time.Second) // Wait for browser to initialize
	
	var buf []byte
	screenshotCtx, screenshotCancel := context.WithTimeout(ctx, 10*time.Second)
	err := chromedp.Run(screenshotCtx, 
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Internal: Executing CaptureScreenshot...")
			return nil
		}),
		chromedp.CaptureScreenshot(&buf),
	)
	screenshotCancel()

	if err != nil {
		emit(EventError, "Failed to capture initial screenshot", err)
		return fmt.Errorf("failed to capture initial screenshot: %w", err)
	}
	
	filename := filepath.Join(outputDir, "initial.png")
	if err := os.WriteFile(filename, buf, 0644); err != nil {
		emit(EventLog, fmt.Sprintf("Warning: failed to save screenshot: %v", err), nil)
	}

	// Debug: Check if DOM is empty
	var dom string
	if err := chromedp.Run(ctx, chromedp.Evaluate("document.body ? document.body.innerText.substring(0, 500) : 'NO BODY'", &dom)); err == nil {
		emit(EventLog, fmt.Sprintf("Initial DOM Content: %q", dom), nil)
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
		
		// Emit history (request) for TUI
		emit(EventRaw, "History Request", map[string]interface{}{"type": "request", "history": history})

		startTime := time.Now()
		// Add a per-call timeout
		genCtx, genCancel := context.WithTimeout(ctx, 90*time.Second)
		resp, err := client.Models.GenerateContent(genCtx, ModelName, history, config)
		genCancel()
		
		duration := time.Since(startTime)
		emit(EventLog, fmt.Sprintf("Model response received in %v", duration.Round(time.Millisecond)), nil)

		if err != nil {
			emit(EventError, fmt.Sprintf("Model call failed: %v", err), nil)
			return fmt.Errorf("generate content failed (after %v): %w", duration, err)
		}

		log.Printf("Received response with %d candidates", len(resp.Candidates))
		// Emit raw response for TUI/Debugging
		emit(EventRaw, "Model Response", resp)

		if len(resp.Candidates) == 0 {
			emit(EventLog, fmt.Sprintf("No candidates returned. Response: %+v", resp), nil)
			break
		}
		cand := resp.Candidates[0]
		emit(EventLog, fmt.Sprintf("Candidate 0 FinishReason: %s", cand.FinishReason), nil)
		
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
				screenshotStart := time.Now()
				
				// Try a few times or check readyState
				err = chromedp.Run(ctx, 
					chromedp.WaitReady("body"),
					chromedp.CaptureScreenshot(&newBuf),
				)
				
				if err != nil {
					emit(EventLog, fmt.Sprintf("Screenshot failed after %v: %v. Retrying with simpler capture...", time.Since(screenshotStart), err), nil)
					// Simple capture fallback
					err = chromedp.Run(ctx, chromedp.CaptureScreenshot(&newBuf))
				}

				                if err != nil {
									emit(EventError, "Failed to capture post-action screenshot", err)
									// Don't just continue with nil buf, provide a placeholder or return error?
									// For now, we MUST have a screenshot for the model's next turn.
								}
				
								// Debug DOM content
								var postDom string
								if err := chromedp.Run(ctx, chromedp.Evaluate("document.body ? document.body.innerText.substring(0, 500) : 'NO BODY'", &postDom)); err == nil {
									emit(EventLog, fmt.Sprintf("Post-Action DOM Content: %q", postDom), nil)
								}
				
								toolResp := &genai.Part{					FunctionResponse: &genai.FunctionResponse{
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

		// Prune old screenshots to save context window
		screenshotsFound := 0
		// Iterate backwards
		for j := len(history) - 1; j >= 0; j-- {
			content := history[j]
			if content.Role != "user" {
				continue
			}

			hasScreenshot := false
			// Check direct InlineData (initial prompt)
			for _, part := range content.Parts {
				if part.InlineData != nil {
					hasScreenshot = true
					break
				}
				// Check FunctionResponse InlineData
				if part.FunctionResponse != nil {
					for _, frPart := range part.FunctionResponse.Parts {
						if frPart.InlineData != nil {
							hasScreenshot = true
							break
						}
					}
				}
			}

			if hasScreenshot {
				screenshotsFound++
				if screenshotsFound > maxScreenshots {
					// Prune!
					for _, part := range content.Parts {
						if part.InlineData != nil {
							part.InlineData = nil // Remove blob
						}
						if part.FunctionResponse != nil {
							for _, frPart := range part.FunctionResponse.Parts {
								if frPart.InlineData != nil {
									frPart.InlineData = nil // Remove blob
								}
							}
						}
					}
				}
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
