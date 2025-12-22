package computer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/cdproto/input"
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
	case "drag_and_drop":
		msg, err = handleDragAndDrop(ctx, call.Args, width, height)
	case "hover", "hover_at":
		msg, err = handleHover(ctx, call.Args, width, height)
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
	
	// Fetch actual URL
	var currentURL string
	if err := chromedp.Run(ctx, chromedp.Evaluate("window.location.href", &currentURL)); err != nil {
		log.Printf("Failed to get current URL: %v", err)
		currentURL = "unknown"
	}
	res["url"] = currentURL
	
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
	
	// 1. Physical Click
	if err := chromedp.Run(ctx, chromedp.MouseClickXY(x, y)); err != nil {
		return nil, err
	}
	
	// 2. JS Focus Fallback (Spatial Aim Assist)
	// If the model clicked a focusable element (input, button), ensure it gets focus.
	// If it missed, find the NEAREST focusable element within a radius.
	var foundTag string
	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function(x, y) {
			const RADIUS = 100;
			let bestCandidate = null;
			let minDist = Infinity;

			// Helper to calc distance from point to rect center
			function getDist(rect) {
				const centerX = rect.left + rect.width / 2;
				const centerY = rect.top + rect.height / 2;
				return Math.hypot(centerX - x, centerY - y);
			}

			// 1. Direct Hit Check
			let hit = document.elementFromPoint(x, y);
			if (hit && (hit.tagName === 'INPUT' || hit.tagName === 'TEXTAREA' || hit.tagName === 'BUTTON' || hit.hasAttribute('tabindex'))) {
				hit.focus();
				if (hit.tagName === 'BUTTON' || (hit.tagName === 'INPUT' && hit.type === 'submit')) {
					hit.click();
					return "DIRECT->" + hit.tagName + " (CLICKED)";
				}
				return "DIRECT->" + hit.tagName;
			}

			// 2. Proximity Search
			const candidates = document.querySelectorAll('input, textarea, button, [tabindex]');
			candidates.forEach(el => {
				const rect = el.getBoundingClientRect();
				// Check if visible
				if (rect.width === 0 || rect.height === 0) return;
				
				const dist = getDist(rect);
				if (dist < RADIUS && dist < minDist) {
					minDist = dist;
					bestCandidate = el;
				}
			});

			if (bestCandidate) {
				bestCandidate.focus();
				// If it's a button, clicking physically might have missed, so help it out.
				if (bestCandidate.tagName === 'BUTTON' || (bestCandidate.tagName === 'INPUT' && bestCandidate.type === 'submit')) {
					bestCandidate.click();
					return "PROXIMITY(" + Math.round(minDist) + "px)->" + bestCandidate.tagName + " (CLICKED)";
				}
				return "PROXIMITY(" + Math.round(minDist) + "px)->" + bestCandidate.tagName;
			}

			return hit ? hit.tagName + " (No Target Found)" : "NONE";
		})(%f, %f)
	`, x, y), &foundTag))
	
	if err == nil {
		log.Printf("JS Focus Result: %s", foundTag)
	}
	
	return "clicked", err
}

func handleType(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	// If coordinates are present, click first
	if args["x"] != nil && args["y"] != nil {
		if _, err := handleMouseClick(ctx, args, width, height); err != nil {
			return nil, err
		}
		// Short wait for focus
		chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
	}
	
	// Verify focus (optional debug)
	var activeTag string
	if err := chromedp.Run(ctx, chromedp.Evaluate("document.activeElement.tagName", &activeTag)); err == nil {
		log.Printf("Active element tag: %s", activeTag)
	}

	text, _ := args["text"].(string)
	if text == "" {
		text, _ = args["value"].(string)
	}
	log.Printf("Typing: %s", text)
	
	// Try physical typing first (optional, or skip to JS if we trust it more)
	// err := chromedp.Run(ctx, chromedp.KeyEvent(text)) 
	
	// Robust method: JS Injection into active element
	// This handles cases where headless focus is wonky or KeyEvents are dropped.
	// We dispatch 'input' and 'change' events so React/Frameworks react to it.
	err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function(txt) {
			const el = document.activeElement;
			if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA')) {
				const start = el.selectionStart || el.value.length;
				const end = el.selectionEnd || el.value.length;
				const original = el.value;
				
				// Insert text at cursor or append
				el.value = original.substring(0, start) + txt + original.substring(end);
				
				// Move cursor
				el.selectionStart = el.selectionEnd = start + txt.length;
				
				// Dispatch events
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
				return "injected";
			}
			return "not_input";
		})(%q)
	`, text), nil))

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

func handleDragAndDrop(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	// Get Start Coords
	x1, y1, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}

	// Get End Coords
	var x2, y2 float64
	if args["destination_x"] != nil && args["destination_y"] != nil {
		x2 = denormalize(args["destination_x"], width)
		y2 = denormalize(args["destination_y"], height)
	} else {
		return nil, fmt.Errorf("missing destination coordinates")
	}

	log.Printf("Dragging from (%f, %f) to (%f, %f)", x1, y1, x2, y2)

	// Execute Drag Sequence using low-level Input domain
	// Move -> Down -> Wait -> Move (Drag) -> Wait -> Up
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Move to start
			if err := input.DispatchMouseEvent(input.MouseMoved, x1, y1).Do(ctx); err != nil {
				return err
			}
			// Mouse Down (Left Button)
			// ClickCount 1 is standard for a click/drag start
			if err := input.DispatchMouseEvent(input.MousePressed, x1, y1).WithButton("left").WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Drag to end
			// buttons=1 means Left Button is held down during move (1 = Left, 2 = Right, 4 = Middle)
			// Wait, WithButtons takes an int.
			if err := input.DispatchMouseEvent(input.MouseMoved, x2, y2).WithButtons(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Mouse Up
			if err := input.DispatchMouseEvent(input.MouseReleased, x2, y2).WithButton("left").WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
	)

	return "dragged", err
}

func handleHover(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}

	log.Printf("Hovering at (%f, %f)", x, y)
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MouseMoved, x, y).Do(ctx)
	}))
	return "hovered", err
}
