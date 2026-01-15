package computer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"google.golang.org/genai"
)

// ToolHandler defines the function signature for a computer tool.
type ToolHandler func(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]ToolHandler)
)

// RegisterTool adds a new tool to the global registry.
func RegisterTool(name string, handler ToolHandler) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = handler
}

// Execute handles the translation of GenAI FunctionCalls to Chromedp actions.
func Execute(ctx context.Context, call *genai.FunctionCall, width, height int) (map[string]interface{}, error) {
	log.Printf("Execute Tool: %s Args: %+v", call.Name, call.Args)

	action := ""
	if call.Name == "computer" {
		if a, ok := call.Args["action"].(string); ok {
			action = a
		}
	} else {
		action = call.Name
	}

	res := map[string]interface{}{"output": nil}

	// Fallback for "url" being passed as a top-level arg
	if action == "" && call.Args["url"] != nil {
		action = "navigate"
	}

	// Handle safety_decision if present
	if safety, ok := call.Args["safety_decision"].(map[string]interface{}); ok {
		log.Printf("SAFETY DECISION: %v", safety)
		res["safety_acknowledgement"] = true
	}

	registryMu.RLock()
	handler, ok := registry[action]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown action: %s", action)
	}

	// Dynamic Timeout Selection (TODO: move timeouts into registry metadata?)
	timeout := 20 * time.Second
	switch action {
	case "navigate", "search":
		timeout = 60 * time.Second
	case "get_page_layout", "scan_page", "get_accessibility_tree":
		timeout = 30 * time.Second
	case "key", "press_key", "hover", "hover_at":
		timeout = 10 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	msg, err := handler(execCtx, call.Args, width, height)
	res["output"] = msg

	// Fetch actual URL
	var currentURL string
	if err := chromedp.Run(ctx, chromedp.Evaluate("window.location.href", &currentURL)); err != nil {
		log.Printf("Failed to get current URL: %v", err)
		currentURL = "unknown"
	}
	res["url"] = currentURL

	return res, err
}