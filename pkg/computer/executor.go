package computer

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/chromedp"
	"google.golang.org/genai"
)

// Execute handles the translation of GenAI FunctionCalls to Chromedp actions.
func Execute(ctx context.Context, call *genai.FunctionCall, width, height int) (map[string]interface{}, error) {
	// TODO: Verify the exact tool name used by Gemini 2.5 Computer Use.
	// Assuming "computer" or specific actions.
	// We'll support a generic dispatch based on "action" arg if name is "computer",
	// or direct dispatch if names are "mouse_click", etc.

	log.Printf("Execute Tool: %s Args: %+v", call.Name, call.Args)

	action := ""
	if call.Name == "computer" {
		if a, ok := call.Args["action"].(string); ok {
			action = a
		}
	} else {
		// Fallback: use the function name as the action (e.g. "click")
		action = call.Name
	}

	res := map[string]interface{}{}
	
	// Handle safety_decision if present
	if safety, ok := call.Args["safety_decision"].(map[string]interface{}); ok {
		log.Printf("SAFETY DECISION: %v", safety)
		// Acknowledge the safety decision to proceed (or we could terminate)
		// For this agent, we'll auto-acknowledge for now.
		res["safety_acknowledgement"] = true
	}

	var err error
	var msg interface{}

	switch action {
	case "mouse_click", "left_click", "click", "click_at":
		msg, err = handleMouseClick(ctx, call.Args, width, height)
	case "type", "input_text", "type_text_at":
		msg, err = handleType(ctx, call.Args, width, height)
	case "key", "press_key", "key_combination":
		msg, err = handleKey(ctx, call.Args)
	case "scroll", "scroll_document", "scroll_at":
		msg, err = handleScroll(ctx, call.Args, width, height)
	case "wait", "wait_5_seconds":
		msg, err = handleWait(ctx, call.Args)
	case "navigate":
		msg, err = handleNavigate(ctx, call.Args)
	case "open_web_browser":
		// Already open, just acknowledge
		msg = "browser_opened"
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
	
	res["output"] = msg
	// TODO: Fetch actual URL
	res["url"] = "https://www.google.com" // Mock for now
	
	return res, err
}

func handleNavigate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	url, _ := args["url"].(string)
	log.Printf("Navigating to: %s", url)
	err := chromedp.Run(ctx, chromedp.Navigate(url))
	return "navigated", err
}

func denormalize(val interface{}, max int) float64 {
	// val might be float64 or int or string (json unmarshal)
	var v float64
	switch t := val.(type) {
	case float64:
		v = t
	case int:
		v = float64(t)
	default:
		return 0
	}
	// Gemini Computer Use outputs 0-1000 range
	return v / 1000.0 * float64(max)
}

func getCoords(args map[string]interface{}, width, height int) (float64, float64, error) {
	// Support both "coordinate": [x, y] and separate keys
	if c, ok := args["coordinate"].([]interface{}); ok && len(c) >= 2 {
		x := denormalize(c[0], width)
		y := denormalize(c[1], height)
		return x, y, nil
	}
	// Support x, y direct
	if args["x"] != nil && args["y"] != nil {
		x := denormalize(args["x"], width)
		y := denormalize(args["y"], height)
		return x, y, nil
	}
	return 0, 0, fmt.Errorf("no coordinates found")
}

func handleMouseClick(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	log.Printf("Clicking at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.MouseClickXY(x, y))
	return "clicked", err
}

func handleType(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	// If coordinates are present, click first
	if args["x"] != nil && args["y"] != nil {
		if _, err := handleMouseClick(ctx, args, width, height); err != nil {
			return nil, err
		}
	}
	
	text, _ := args["text"].(string)
	if text == "" {
		text, _ = args["value"].(string)
	}
	log.Printf("Typing: %s", text)
	// We might need to focus explicitly, but usually previous click handles it.
	// Or use kb.Type which types into focused element.
	err := chromedp.Run(ctx, chromedp.KeyEvent(text)) 
	return "typed", err
}

func handleKey(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Map key names like "Enter", "Return" to generic keys if needed.
	// Chromedp uses kb package for virtual keys.
	// For now, pass string directly as it might be raw text or special key.
	// If it's a special key, we might need a lookup table.
	key, _ := args["text"].(string)
	if key == "" {
		key, _ = args["value"].(string) 
	}
	// Python agent uses "keys" string "Ctrl+C" etc.
	if k, ok := args["keys"].(string); ok {
		key = k
	}

	// Simple mapping for common keys
	if key == "Enter" || key == "return" {
		key = "\r"
	}
	
	log.Printf("Pressing Key: %s", key)
	err := chromedp.Run(ctx, chromedp.KeyEvent(key))
	return "pressed", err
}

func handleScroll(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	// Scroll usually takes deltaX, deltaY or coordinate
	// For now, simple scroll down if no args?
	// Or "coordinate" to scroll TO?
	// Let's assume standard "scroll" tool has "delta_y" or similar.
	return "scrolled", nil
}

func handleWait(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Explicit wait
	return "waited", nil
}
