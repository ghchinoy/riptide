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

package computer

import (
	"context"
	"encoding/json"
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

func Run(ctx context.Context, client *genai.Client, sessionsDir, sessionID, prompt string, makeGif, showBrowser bool, userAgent string, useAXT bool, observer Observer, safetyHandler SafetyHandler, maxTurns, maxScreenshots int, mode string) error {
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
	sessionPath := filepath.Join(sessionsDir, sessionID)
	outputDir := filepath.Join(sessionPath, "screenshots")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// 1. Setup Chromedp
	emit(EventStatus, "Initializing browser allocator...", nil)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1280, 1024),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.UserAgent(userAgent),
	)
	if showBrowser {
		opts = append(opts, chromedp.Flag("headless", false))
	} else {
		opts = append(opts, chromedp.Headless)
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	emit(EventStatus, "Creating browser context...", nil)
	ctx, cancel = chromedp.NewContext(allocCtx,
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// Ensure the target is initialized with the long-lived context
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return nil
	})); err != nil {
		return fmt.Errorf("failed to initialize browser: %w", err)
	}

	// Configure the model
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

	// Add any custom skills to the tools schema
	customDecls := GetCustomSkillDeclarations()
	if len(customDecls) > 0 {
		config.Tools = append(config.Tools, &genai.Tool{
			FunctionDeclarations: customDecls,
		})
	}

	// 3. Pre-flight Diagnostics
	emit(EventStatus, "Running pre-flight diagnostics...", nil)
	// Create a sub-context just for pre-flight
	pfCtx, pfCancel := context.WithTimeout(ctx, 30*time.Second)

	if err := chromedp.Run(pfCtx, chromedp.Navigate("about:blank")); err != nil {
		pfCancel()
		return fmt.Errorf("ENVIRONMENT NOT READY: failed to launch browser or navigate to blank page: %w. Please ensure Chrome/Chromium is installed and accessible", err)
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

	buf, err := captureInitialScreenshot(ctx, outputDir, emit)
	if err != nil {
		return err
	}

	// Debug: Check if DOM is empty
	var dom string
	domCtx, domCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := chromedp.Run(domCtx, chromedp.Evaluate("document.body ? document.body.innerText.substring(0, 500) : 'NO BODY'", &dom)); err == nil {
		emit(EventLog, fmt.Sprintf("Initial DOM Content: %q", dom), nil)
	}
	domCancel()

	history[0].Parts = append(history[0].Parts, &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "image/png",
			Data:     buf,
		},
	})

	// Capture initial AXTree

	if useAXT {

		axTree, err := handleGetAccessibilityTree(ctx, nil, 1280, 1024)

		if err == nil {

			if b, err := json.Marshal(axTree); err == nil {

				history[0].Parts = append(history[0].Parts, &genai.Part{

					Text: fmt.Sprintf("Accessibility Tree (Semantic View):\n%s", string(b)),
				})

			}

		}

	}

	defer func() {
		if makeGif {
			emit(EventStatus, "Generating GIF...", nil)
			gifPath := filepath.Join(sessionPath, "session.gif")
			cmd := exec.Command("ffmpeg",
				"-framerate", "1",
				"-i", filepath.Join(outputDir, "turn_%d_post.png"),
				"-y",
				gifPath,
			)
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
				
				// 1. Detect Hallucination before Execution
				actionName := part.FunctionCall.Name
				if actionName == "computer" {
					if a, ok := part.FunctionCall.Args["action"].(string); ok {
						actionName = a
					}
				}

				if !IsToolKnown(actionName) {
					emit(EventHallucination, fmt.Sprintf("Hallucinated Tool: %s", actionName), part.FunctionCall)
					log.Printf("Intercepted hallucinated tool call: %s", actionName)
					
					// We do not execute it. We must also drop the FunctionCall from the history 
					// so Vertex AI doesn't reject the next request with a 400.
					
					// Instead of a FunctionResponse (which Vertex would validate and reject), 
					// we will inject a synthetic text prompt correcting the model.
					
					// Pop the hallucinated cand.Content from history (which we just appended)
					history = history[:len(history)-1]
					
					// Append a correction message directly to history
					correctionMsg := fmt.Sprintf("System Error: You attempted to use an invalid tool '%s'. Please only use tools explicitly provided in your configuration. Do not hallucinate tools like 'go_back', 'scroll_down', etc. Use the provided tools (e.g. 'computer' action='scroll_document' or 'navigate').", actionName)
					history = append(history, &genai.Content{
						Role: "user",
						Parts: []*genai.Part{
							{Text: correctionMsg},
						},
					})
					
					emit(EventLog, "Injected synthetic correction prompt for hallucinated tool", nil)
					// Break out of the parts loop so we don't try to process this hallucination further
					// The main loop will continue to the next turn and send the correction.
					continue
				}

				emit(EventAction, fmt.Sprintf("Tool Call: %s", part.FunctionCall.Name), part.FunctionCall.Args)

				resultMap, err := Execute(ctx, part.FunctionCall, 1280, 1024)
				if err != nil {
					log.Printf("Execute error: %v", err)
				}

				// Handle Safety Interaction if present
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
				newBuf, err := capturePostActionScreenshot(ctx, i, outputDir, emit)
				if err != nil {
					emit(EventError, "Failed to capture post-action screenshot", err)
				}

				// Debug DOM content
				var postDom string
				evalCtx, evalCancel := context.WithTimeout(ctx, 5*time.Second)
				if err := chromedp.Run(evalCtx, chromedp.Evaluate("document.body ? document.body.innerText.substring(0, 500) : 'NO BODY'", &dom)); err == nil {
					postDom = dom
					emit(EventLog, fmt.Sprintf("Post-Action DOM Content: %q", postDom), nil)
				}
				evalCancel()

				toolResp := &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     part.FunctionCall.Name,
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

								// Capture AXTree for this turn

								if useAXT {

									axTree, err := handleGetAccessibilityTree(ctx, nil, 1280, 1024)

									if err == nil {

										if b, err := json.Marshal(axTree); err == nil {

											history = append(history, &genai.Content{

												Role: "user",

												Parts: []*genai.Part{

													{

														Text: fmt.Sprintf("Accessibility Tree (Semantic View):\n%s", string(b)),

													},

												},

											})

										}

									}

								}

				

			}
		}
		// Prune old screenshots to save context window
		pruneOldScreenshots(history, maxScreenshots)

		if !hasToolCalls {
			emit(EventStatus, "Goal Achieved.", nil)
			break
		}
		if i == maxTurns-1 {
			emit(EventStatus, "Max Turns Reached.", nil)
		}
	}

	return nil
}

func pruneOldScreenshots(history []*genai.Content, maxScreenshots int) {
	screenshotsFound := 0
	for j := len(history) - 1; j >= 0; j-- {
		content := history[j]
		if content.Role != "user" {
			continue
		}

		hasScreenshot := false
		for _, part := range content.Parts {
			if part.InlineData != nil {
				hasScreenshot = true
				break
			}
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
				for _, part := range content.Parts {
					if part.InlineData != nil {
						part.InlineData = nil
					}
					if part.FunctionResponse != nil {
						for _, frPart := range part.FunctionResponse.Parts {
							if frPart.InlineData != nil {
								frPart.InlineData = nil
							}
						}
					}
				}
			}
		}
	}
}

func capturePostActionScreenshot(ctx context.Context, i int, outputDir string, emit func(EventType, string, interface{})) ([]byte, error) {
	var newBuf []byte
	screenshotStart := time.Now()

	log.Printf("Taking post-action screenshot for turn %d...", i+1)
	screenshotCtx, screenshotCancel := context.WithTimeout(ctx, 15*time.Second)
	err := chromedp.Run(screenshotCtx,
		chromedp.WaitReady("body"),
		chromedp.CaptureScreenshot(&newBuf),
	)
	screenshotCancel()

	if err != nil {
		emit(EventLog, fmt.Sprintf("Screenshot failed after %v: %v. Retrying with simpler capture...", time.Since(screenshotStart), err), nil)
		fallbackCtx, fallbackCancel := context.WithTimeout(ctx, 5*time.Second)
		err = chromedp.Run(fallbackCtx, chromedp.CaptureScreenshot(&newBuf))
		fallbackCancel()
	}

	if err == nil {
		log.Printf("Post-action screenshot captured: %d bytes", len(newBuf))
		postFilename := filepath.Join(outputDir, fmt.Sprintf("turn_%d_post.png", i+1))
		if err := os.WriteFile(postFilename, newBuf, 0644); err != nil {
			log.Printf("Warning: failed to save post-action screenshot: %v", err)
		}

		var fullBuf []byte
		if err := captureFullPageScreenshot(ctx, &fullBuf); err == nil {
			fullFilename := filepath.Join(outputDir, fmt.Sprintf("turn_%d_full.png", i+1))
			_ = os.WriteFile(fullFilename, fullBuf, 0644)
		}
	}
	return newBuf, err
}

func captureInitialScreenshot(ctx context.Context, outputDir string, emit func(EventType, string, interface{})) ([]byte, error) {
	log.Println("Taking initial screenshot...")

	var buf []byte
	initialCtx, initialCancel := context.WithTimeout(ctx, 10*time.Second)
	err := chromedp.Run(initialCtx,
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("Internal: Browser ready, capturing initial screenshot...")
			return nil
		}),
		chromedp.CaptureScreenshot(&buf),
	)
	initialCancel()

	if err != nil {
		emit(EventLog, fmt.Sprintf("Initial screenshot failed: %v. Retrying with simpler capture...", err), nil)
		screenshotCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = chromedp.Run(screenshotCtx, chromedp.CaptureScreenshot(&buf))
		cancel()
	}

	if err != nil {
		emit(EventError, "Failed to capture initial screenshot", err)
		return nil, fmt.Errorf("failed to capture initial screenshot: %w", err)
	}
	log.Printf("Initial screenshot captured: %d bytes", len(buf))

	filename := filepath.Join(outputDir, "initial.png")
	if err := os.WriteFile(filename, buf, 0644); err != nil {
		emit(EventLog, fmt.Sprintf("Warning: failed to save screenshot: %v", err), nil)
	}
	return buf, nil
}
