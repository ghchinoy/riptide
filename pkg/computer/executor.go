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

	// Fallback for "url" being passed as a top-level arg (sometimes happens with raw model calls)
	if action == "" && call.Args["url"] != nil {
		action = "navigate"
	}

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
		execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		msg, err = handleMouseClick(execCtx, call.Args, width, height)
		cancel()
	case "type", "input_text", "type_text_at":
		execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		msg, err = handleType(execCtx, call.Args, width, height)
		cancel()
	case "key", "press_key", "key_combination":
		execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		msg, err = handleKey(execCtx, call.Args)
		cancel()
	case "scroll", "scroll_document", "scroll_at":
		execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		msg, err = handleScroll(execCtx, call.Args, width, height)
		cancel()
	case "drag_and_drop":
		execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		msg, err = handleDragAndDrop(execCtx, call.Args, width, height)
		cancel()
	case "hover", "hover_at":
		execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		msg, err = handleHover(execCtx, call.Args, width, height)
		cancel()
	case "wait", "wait_5_seconds":
		msg, err = handleWait(ctx, call.Args) // handleWait has its own sleep
	case "get_computed_style", "inspect_element":
		execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		msg, err = handleGetComputedStyle(execCtx, call.Args, width, height)
		cancel()
	case "get_page_layout", "scan_page":
		execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		msg, err = handleGetPageLayout(execCtx, call.Args, width, height)
		cancel()
	case "navigate":
		execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		msg, err = handleNavigate(execCtx, call.Args)
		cancel()
	case "search":
		// Handle hallucinated 'search' tool by navigating to google
		execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		args := map[string]interface{}{"url": "https://www.google.com"}
		msg, err = handleNavigate(execCtx, args)
		cancel()
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

	// Diagnostic: log current window info

	var winInfo string

	if err := chromedp.Run(ctx, chromedp.Evaluate(`"win:" + window.innerWidth + "x" + window.innerHeight + " vp:" + document.documentElement.clientWidth + "x" + document.documentElement.clientHeight`, &winInfo)); err == nil {

		log.Printf("Dimension Info: %s", winInfo)

	}

	// Diagnostic: what is at these coords?

	var elementAt string

	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("document.elementFromPoint(%f, %f)?.tagName || 'NONE'", x, y), &elementAt)); err == nil {

		log.Printf("Element at click coords: %s", elementAt)

	}

	// 1. Physical Click

	if err := chromedp.Run(ctx, chromedp.MouseClickXY(x, y)); err != nil {

		return nil, err

	}

	// Small wait for effects

	chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))

	// 2. JS Focus Fallback (Spatial Aim Assist)

	// If the model clicked a focusable element (input, button), ensure it gets focus.

	// If it missed, find the NEAREST focusable element within a radius.

	var foundTag string

	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`

			(function(x, y) {

				const RADIUS = 100;

				let bestCandidate = null;

				let minDist = Infinity;

	

				function getDeepElement(x, y) {

					let el = document.elementFromPoint(x, y);

					while (el && el.shadowRoot) {

						const inner = el.shadowRoot.elementFromPoint(x, y);

						if (!inner || inner === el) break;

						el = inner;

					}

					return el;

				}

	

				function getDist(rect) {

					const centerX = rect.left + rect.width / 2;

					const centerY = rect.top + rect.height / 2;

					return Math.hypot(centerX - x, centerY - y);

				}

	

				// 1. Direct Hit Check (Deep)

				let hit = getDeepElement(x, y);

				if (hit && (hit.tagName === 'INPUT' || hit.tagName === 'TEXTAREA' || hit.tagName === 'BUTTON' || hit.hasAttribute('tabindex'))) {

					hit.focus();

					if (hit.tagName === 'BUTTON' || (hit.tagName === 'INPUT' && (hit.type === 'submit' || hit.type === 'range'))) {

						// hit.click(); // Be careful with double-clicks

						return "DIRECT->" + hit.tagName + " (FOCUSED)";

					}

					return "DIRECT->" + hit.tagName;

				}

	

				// 2. Proximity Search

				const candidates = document.querySelectorAll('input, textarea, button, [tabindex], [role="button"], [role="slider"]');

				candidates.forEach(el => {

					const rect = el.getBoundingClientRect();

					if (rect.width === 0 || rect.height === 0) return;

					

					const dist = getDist(rect);

					if (dist < RADIUS && dist < minDist) {

						minDist = dist;

						bestCandidate = el;

					}

				});

	

				if (bestCandidate) {

					bestCandidate.focus();

					// Use center-scrolling to ensure it stays in view but ideally not jumpy

					bestCandidate.scrollIntoView({ behavior: 'instant', block: 'center', inline: 'center' });

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
	// Gemini Computer Use often uses scroll_at or scroll
	// with delta_x, delta_y or a target coordinate.
	dx := 0.0
	dy := 0.0

	if v, ok := args["delta_x"].(float64); ok {
		dx = v
	}
	if v, ok := args["delta_y"].(float64); ok {
		dy = v
	}

	// Handle semantic directions (riptide-bz4)
	if direction, ok := args["direction"].(string); ok {
		switch direction {
		case "down":
			dy = 500
		case "up":
			dy = -500
		case "left":
			dx = -500
		case "right":
			dx = 500
		}
	}

	// Handle normalized scrolling if delta is large (e.g. 0-1000)
	// But usually scrollBy is in pixels.
	// If the model sends normalized coordinates, we should denormalize.
	// However, scroll deltas are often relative pixels.

	// If coordinates are provided, scroll that point into view or scroll TO it.
	if args["x"] != nil && args["y"] != nil {
		x, y, _ := getCoords(args, width, height)
		log.Printf("Scrolling to coordinate: %f, %f", x, y)
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("window.scrollTo(%f, %f)", x, y), nil))
		return "scrolled_to_coords", err
	}

	log.Printf("Scrolling by delta: dx=%f, dy=%f", dx, dy)
	err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("window.scrollBy({top: %f, left: %f, behavior: 'smooth'})", dy, dx), nil))
	if err != nil {
		return nil, err
	}
	// Wait for smooth scroll
	err = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	return "scrolled", err
}

func handleWait(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	seconds := 2.0 // default
	if s, ok := args["seconds"].(float64); ok {
		seconds = s
	} else if s, ok := args["value"].(float64); ok {
		seconds = s
	}

	log.Printf("Waiting for %f seconds", seconds)
	err := chromedp.Run(ctx, chromedp.Sleep(time.Duration(seconds)*time.Second))
	return "waited", err
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

	// Diagnostic: log elements at start and end
	var elementStart string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("document.elementFromPoint(%f, %f)?.tagName || 'NONE'", x1, y1), &elementStart)); err == nil {
		log.Printf("Element at drag start: %s", elementStart)
	}
	var elementEnd string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("document.elementFromPoint(%f, %f)?.tagName || 'NONE'", x2, y2), &elementEnd)); err == nil {
		log.Printf("Element at drag end: %s", elementEnd)
	}

	// Execute Drag Sequence
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Dispatching MouseMoved to start")
			if err := input.DispatchMouseEvent(input.MouseMoved, x1, y1).Do(ctx); err != nil {
				return err
			}
			log.Printf("Dispatching MousePressed")
			if err := input.DispatchMouseEvent(input.MousePressed, x1, y1).WithButton("left").WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Dispatching MouseMoved to end")
			if err := input.DispatchMouseEvent(input.MouseMoved, x2, y2).WithButtons(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Dispatching MouseReleased")
			if err := input.DispatchMouseEvent(input.MouseReleased, x2, y2).WithButton("left").WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
	)
	// Small wait for effects
	chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))

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

func handleGetComputedStyle(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}

	log.Printf("Inspecting element at (%f, %f)", x, y)

	var style map[string]interface{}
	err = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function(x, y) {
			const el = document.elementFromPoint(x, y);
			if (!el) return { error: "no element at coordinates" };
			
			const computed = window.getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			
			return {
				tagName: el.tagName,
				id: el.id,
				className: el.className,
				value: el.value,
				innerText: el.innerText,
				ariaValueNow: el.getAttribute('aria-valuenow'),
				ariaLabel: el.getAttribute('aria-label'),
				computedStyle: {
					margin: computed.margin,
					padding: computed.padding,
					color: computed.color,
					backgroundColor: computed.backgroundColor,
					display: computed.display,
					visibility: computed.visibility,
					opacity: computed.opacity,
					border: computed.border
				},
				rect: {
					top: rect.top,
					left: rect.left,
					width: rect.width,
					height: rect.height
				}
			};
		})(%f, %f)
	`, x, y), &style))

	return style, err
}

func handleGetPageLayout(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	log.Printf("Scanning page for interactive elements...")

	var elements []interface{}
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			const results = [];
			const interactive = document.querySelectorAll('button, input, textarea, a, [role="button"], [role="checkbox"], [role="slider"], [tabindex]');
			
			interactive.forEach(el => {
				const rect = el.getBoundingClientRect();
				if (rect.width === 0 || rect.height === 0) return;
				
				// Skip if far outside viewport if we want to be strict, 
				// but let's return everything so model knows it needs to scroll.
				
				results.push({
					tagName: el.tagName,
					id: el.id,
					className: el.className,
					innerText: el.innerText || el.ariaLabel || el.placeholder || el.value || "unlabeled",
					role: el.getAttribute('role'),
					type: el.getAttribute('type'),
					rect: {
						x: Math.round(rect.left),
						y: Math.round(rect.top),
						width: Math.round(rect.width),
						height: Math.round(rect.height)
					},
					// Normalized for Gemini (0-1000)
					center_normalized: [
						Math.round((rect.left + rect.width / 2) / window.innerWidth * 1000),
						Math.round((rect.top + rect.height / 2) / window.innerHeight * 1000)
					]
				});
			});
			return results;
		})()
	`, &elements))

	return elements, err
}
