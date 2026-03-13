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

type CustomSkill struct {
	Declaration *genai.FunctionDeclaration
	Handler     ToolHandler
}

var (
	registryMu      sync.RWMutex
	registry        = make(map[string]ToolHandler)
	customSkills    []*CustomSkill
)

// RegisterTool adds a new core tool to the global registry.
func RegisterTool(name string, handler ToolHandler) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = handler
}

// RegisterCustomSkill registers a tool that extends the base Vertex AI Computer Use schema
func RegisterCustomSkill(skill *CustomSkill) {
	registryMu.Lock()
	defer registryMu.Unlock()
	
	// Register the handler
	registry[skill.Declaration.Name] = skill.Handler
	customSkills = append(customSkills, skill)
}

// GetCustomSkillDeclarations returns all registered custom skill declarations
func GetCustomSkillDeclarations() []*genai.FunctionDeclaration {
	registryMu.RLock()
	defer registryMu.RUnlock()
	
	decls := make([]*genai.FunctionDeclaration, len(customSkills))
	for i, skill := range customSkills {
		decls[i] = skill.Declaration
	}
	return decls
}

// IsToolKnown checks if a tool exists in the registry without executing it.
func IsToolKnown(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
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

	// Anti-Hallucination Alias Mapper
	aliases := map[string]string{
		"scroll_down": "scroll",
		"scroll_up":   "scroll",
		"search":      "navigate", // We map hallucinated searches to navigate
	}
	if mapped, ok := aliases[action]; ok {
		log.Printf("Intercepted alias/hallucination: mapping %s -> %s", action, mapped)
		
		// If it's a scroll alias, try to inject the direction argument if it doesn't exist
		if action == "scroll_down" && call.Args["direction"] == nil {
			call.Args["direction"] = "down"
		} else if action == "scroll_up" && call.Args["direction"] == nil {
			call.Args["direction"] = "up"
		}
		
		action = mapped
		// Note: The history mutation (which prevents the 400 error)
		// must happen in computer.go during the pre-execution phase, 
		// but since `go_back` is now formally registered, it shouldn't need mapping anyway.
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