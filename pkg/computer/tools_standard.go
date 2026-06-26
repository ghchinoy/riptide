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
	"fmt"
	"log"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

func init() {
	// ── Click family ────────────────────────────────────────────────────────────
	// 2.5 names: click_at, left_click, mouse_click
	// 3.5 names: click, double_click, triple_click, middle_click, right_click
	RegisterTool("mouse_click", handleMouseClick)
	RegisterTool("left_click", handleMouseClick)
	RegisterTool("click", handleMouseClick)    // 3.5
	RegisterTool("click_at", handleMouseClick) // 2.5

	RegisterTool("right_click", handleRightClick)
	RegisterTool("middle_click", handleMiddleClick)
	RegisterTool("double_click", handleDoubleClick)
	RegisterTool("triple_click", handleTripleClick) // 3.5 new

	// ── Mouse movement ──────────────────────────────────────────────────────────
	// 2.5 names: mouse_move, hover_at
	// 3.5 names: move
	RegisterTool("mouse_move", handleMouseMove)
	RegisterTool("move", handleMouseMove) // 3.5
	RegisterTool("cursor_position", handleCursorPosition)

	// ── Mouse button press/release (3.5 new) ───────────────────────────────────
	RegisterTool("mouse_down", handleMouseDown) // 3.5
	RegisterTool("mouse_up", handleMouseUp)     // 3.5

	// ── Text input ──────────────────────────────────────────────────────────────
	// 2.5 names: type_text_at, input_text
	// 3.5 names: type (no coords — uses active element)
	RegisterTool("type", handleType)          // 3.5 (also handles 2.5 with x/y)
	RegisterTool("input_text", handleType)    // 2.5
	RegisterTool("type_text_at", handleType)  // 2.5

	// ── Keyboard ────────────────────────────────────────────────────────────────
	// 2.5 names: key, key_combination
	// 3.5 names: press_key, hotkey, key_down, key_up
	RegisterTool("key", handleKey)
	RegisterTool("press_key", handleKey)         // 3.5 (single key)
	RegisterTool("key_combination", handleKey)   // 2.5
	RegisterTool("hotkey", handleKey)            // 3.5 (modifier combos e.g. "ctrl+c")
	RegisterTool("key_down", handleKeyDown)      // 3.5 new (press and hold)
	RegisterTool("key_up", handleKeyUp)          // 3.5 new (release held key)

	// ── Scroll ──────────────────────────────────────────────────────────────────
	// 2.5 names: scroll_document, scroll_at
	// 3.5 names: scroll (with magnitude arg in pixels)
	RegisterTool("scroll", handleScroll)
	RegisterTool("scroll_document", handleScroll) // 2.5
	RegisterTool("scroll_at", handleScroll)       // 2.5

	// ── Drag ────────────────────────────────────────────────────────────────────
	RegisterTool("drag_and_drop", handleDragAndDrop)

	// ── Hover ───────────────────────────────────────────────────────────────────
	RegisterTool("hover", handleHover)
	RegisterTool("hover_at", handleHover) // 2.5

	// ── Wait ────────────────────────────────────────────────────────────────────
	RegisterTool("wait", handleWait)
	RegisterTool("wait_5_seconds", handleWait) // 2.5

	// ── Screenshot (3.5 new explicit tool) ─────────────────────────────────────
	// The harness captures a screenshot after every action automatically;
	// this no-op handler lets the model call take_screenshot without crashing.
	RegisterTool("take_screenshot", func(_ context.Context, _ map[string]interface{}, _, _ int) (interface{}, error) {
		log.Printf("take_screenshot: screenshot will be captured by the harness after this turn")
		return "screenshot_captured_by_harness", nil
	})

	// ── Inspection / Layout ─────────────────────────────────────────────────────
	RegisterTool("get_computed_style", handleGetComputedStyle)
	RegisterTool("inspect_element", handleGetComputedStyle)
	RegisterTool("get_page_layout", handleGetPageLayout)
	RegisterTool("scan_page", handleGetPageLayout)
	RegisterTool("get_accessibility_tree", handleGetAccessibilityTree)

	// ── Navigation ──────────────────────────────────────────────────────────────
	RegisterTool("navigate", handleNavigateWrapper)
	RegisterTool("search", handleSearch)

	RegisterTool("open_web_browser", func(_ context.Context, _ map[string]interface{}, _, _ int) (interface{}, error) {
		return "browser_opened", nil
	})

	RegisterTool("go_back", func(ctx context.Context, _ map[string]interface{}, _, _ int) (interface{}, error) {
		log.Printf("Executing go_back")
		err := chromedp.Run(ctx, chromedp.NavigateBack())
		return "navigated_back", err
	})

	RegisterTool("go_forward", func(ctx context.Context, _ map[string]interface{}, _, _ int) (interface{}, error) { // 3.5 new
		log.Printf("Executing go_forward")
		// NavigateForward can hang if there is no forward history; use JS instead
		// so it is non-blocking — the browser simply stays on the current page.
		err := chromedp.Run(ctx, chromedp.Evaluate("window.history.forward()", nil))
		return "navigated_forward", err
	})
}

func handleNavigateWrapper(ctx context.Context, args map[string]interface{}, w, h int) (interface{}, error) {
	return handleNavigate(ctx, args)
}

func handleNavigate(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	url, _ := args["url"].(string)
	log.Printf("Navigating to: %s", url)
	err := chromedp.Run(ctx, chromedp.Navigate(url))
	return "navigated", err
}

func handleSearch(ctx context.Context, args map[string]interface{}, w, h int) (interface{}, error) {
	// Handle hallucinated 'search' tool by navigating to google
	searchArgs := map[string]interface{}{ "url": "https://www.google.com"}
	return handleNavigate(ctx, searchArgs)
}

func denormalize(val interface{}, max int) float64 {
	var v float64
	switch t := val.(type) {
	case float64:
		v = t
	case int:
		v = float64(t)
	default:
		return 0
	}
	return v / 1000.0 * float64(max)
}

func getCoords(args map[string]interface{}, width, height int) (float64, float64, error) {
	if c, ok := args["coordinate"].([]interface{}); ok && len(c) >= 2 {
		x := denormalize(c[0], width)
		y := denormalize(c[1], height)
		return x, y, nil
	}
	if args["x"] != nil && args["y"] != nil {
		x := denormalize(args["x"], width)
		y := denormalize(args["y"], height)
		return x, y, nil
	}
	return 0, 0, fmt.Errorf("no coordinates found")
}

func handleRightClick(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Right clicking at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.MouseClickXY(x, y, chromedp.ButtonType(input.Right)))
	return "right_clicked", err
}

func handleMiddleClick(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Middle clicking at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.MouseClickXY(x, y, chromedp.ButtonType(input.Middle)))
	return "middle_clicked", err
}

func handleDoubleClick(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Double clicking at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.MouseClickXY(x, y, chromedp.ClickCount(2)))
	return "double_clicked", err
}

// handleTripleClick is a Gemini 3.5 Flash new action (select-all equivalent on text fields).
func handleTripleClick(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Triple clicking at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.MouseClickXY(x, y, chromedp.ClickCount(3)))
	return "triple_clicked", err
}

// handleMouseDown presses the mouse button without releasing it (3.5 new — used for drag start).
func handleMouseDown(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Mouse down at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MousePressed, x, y).WithButton("left").WithClickCount(1).Do(ctx)
	}))
	return "mouse_down", err
}

// handleMouseUp releases the mouse button (3.5 new — used for drag end).
func handleMouseUp(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Mouse up at %f, %f", x, y)
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MouseReleased, x, y).WithButton("left").WithClickCount(1).Do(ctx)
	}))
	return "mouse_up", err
}

// handleKeyDown presses and holds a key (3.5 new).
func handleKeyDown(ctx context.Context, args map[string]interface{}, _, _ int) (interface{}, error) {
	key := resolveKeyArg(args)
	log.Printf("Key down: %s", key)
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchKeyEvent(input.KeyDown).WithKey(key).Do(ctx)
	}))
	return "key_down", err
}

// handleKeyUp releases a held key (3.5 new).
func handleKeyUp(ctx context.Context, args map[string]interface{}, _, _ int) (interface{}, error) {
	key := resolveKeyArg(args)
	log.Printf("Key up: %s", key)
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchKeyEvent(input.KeyUp).WithKey(key).Do(ctx)
	}))
	return "key_up", err
}

// resolveKeyArg extracts the key string from action args, supporting all arg name variants.
func resolveKeyArg(args map[string]interface{}) string {
	for _, field := range []string{"key", "text", "value", "keys"} {
		if v, ok := args[field].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func handleMouseMove(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	// mouse_move is functionally identical to hover in our implementation
	return handleHover(ctx, args, width, height)
}

func handleCursorPosition(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	// CDP doesn't have a reliable way to query absolute mouse position on demand without 
	// injecting tracking into the DOM ahead of time.
	// Since we are the only thing moving the mouse via tools, we track the last dispatched coordinate.
	normalized := []int{
		int((lastMouseX / float64(width)) * 1000),
		int((lastMouseY / float64(height)) * 1000),
	}
	return normalized, nil
}

var (
	lastMouseX float64
	lastMouseY float64
)

func handleMouseClick(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
	log.Printf("Clicking at %f, %f", x, y)
	var winInfo string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`"win:" + window.innerWidth + "x" + window.innerHeight + " vp:" + document.documentElement.clientWidth + "x" + document.documentElement.clientHeight`, &winInfo)); err == nil {
		log.Printf("Dimension Info: %s", winInfo)
	}
	var elementAt string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("document.elementFromPoint(%f, %f)?.tagName || 'NONE'", x, y), &elementAt)); err == nil {
		log.Printf("Element at click coords: %s", elementAt)
	}
	if err := chromedp.Run(ctx, chromedp.MouseClickXY(x, y)); err != nil {
		return nil, err
	}
	_ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
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
				let hit = getDeepElement(x, y);
				if (hit && (hit.tagName === 'INPUT' || hit.tagName === 'TEXTAREA' || hit.tagName === 'BUTTON' || hit.hasAttribute('tabindex'))) {
					hit.focus();
					if (hit.tagName === 'BUTTON' || (hit.tagName === 'INPUT' && (hit.type === 'submit' || hit.type === 'range'))) {
						return "DIRECT->" + hit.tagName + " (FOCUSED)";
					}
					return "DIRECT->" + hit.tagName;
				}
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
	if args["x"] != nil && args["y"] != nil {
		if _, err := handleMouseClick(ctx, args, width, height); err != nil {
			return nil, err
		}
		_ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
	}
	var activeTag string
	if err := chromedp.Run(ctx, chromedp.Evaluate("document.activeElement.tagName", &activeTag)); err == nil {
		log.Printf("Active element tag: %s", activeTag)
	}
	text, _ := args["text"].(string)
	if text == "" {
		text, _ = args["value"].(string)
	}
	log.Printf("Typing: %s", text)
	err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
		(function(txt) {
			const el = document.activeElement;
			if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA')) {
				const start = el.selectionStart || el.value.length;
				const end = el.selectionEnd || el.value.length;
				const original = el.value;
				el.value = original.substring(0, start) + txt + original.substring(end);
				el.selectionStart = el.selectionEnd = start + txt.length;
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
				return "injected";
			}
			return "not_input";
		})(%q)
	`, text), nil))
	return "typed", err
}

func handleKey(ctx context.Context, args map[string]interface{}, _, _ int) (interface{}, error) {
	key := resolveKeyArg(args)
	if key == "Enter" || key == "return" {
		key = "\r"
	}
	log.Printf("Pressing Key: %s", key)
	err := chromedp.Run(ctx, chromedp.KeyEvent(key))
	return "pressed", err
}

func handleScroll(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	dx := 0.0
	dy := 0.0

	// 2.5-style: delta_x / delta_y
	if v, ok := args["delta_x"].(float64); ok {
		dx = v
	}
	if v, ok := args["delta_y"].(float64); ok {
		dy = v
	}

	// 3.5-style: direction + magnitude (pixels, denormalized).
	// Default magnitude matches the old fixed 500 px.
	if direction, ok := args["direction"].(string); ok {
		magnitude := 500.0
		if m, ok := args["magnitude"].(float64); ok && m > 0 {
			// The reference impl denormalizes magnitude the same way as coordinates.
			switch direction {
			case "up", "down":
				magnitude = denormalize(m, height)
			case "left", "right":
				magnitude = denormalize(m, width)
			}
		}
		switch direction {
		case "down":
			dy = magnitude
		case "up":
			dy = -magnitude
		case "left":
			dx = -magnitude
		case "right":
			dx = magnitude
		}
	}

	// scroll_at / scroll_document with explicit x,y target coordinates (2.5-style)
	if args["x"] != nil && args["y"] != nil && dx == 0 && dy == 0 {
		x, y, _ := getCoords(args, width, height)
		log.Printf("Scrolling to coordinate: %f, %f", x, y)
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("window.scrollTo(%f, %f)", x, y), nil))
		return "scrolled_to_coords", err
	}

	log.Printf("Scrolling by delta: dx=%f, dy=%f", dx, dy)
	err := chromedp.Run(ctx, chromedp.Evaluate(
		fmt.Sprintf("window.scrollBy({top: %f, left: %f, behavior: 'smooth'})", dy, dx), nil))
	if err != nil {
		return nil, err
	}
	err = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	return "scrolled", err
}

func handleWait(ctx context.Context, args map[string]interface{}, _, _ int) (interface{}, error) {
	seconds := 2.0
	if s, ok := args["seconds"].(float64); ok {
		seconds = s
	} else if s, ok := args["value"].(float64); ok {
		seconds = s
	}
	log.Printf("Waiting for %f seconds", seconds)
	// Use float64 multiplication before converting to Duration to avoid truncation
	// of sub-second values (e.g. 0.1s → 100ms, not 0s).
	err := chromedp.Run(ctx, chromedp.Sleep(time.Duration(seconds*float64(time.Second))))
	return "waited", err
}

func handleDragAndDrop(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x1, y1, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	var x2, y2 float64
	if args["destination_x"] != nil && args["destination_y"] != nil {
		x2 = denormalize(args["destination_x"], width)
		y2 = denormalize(args["destination_y"], height)
	} else {
		return nil, fmt.Errorf("missing destination coordinates")
	}
	log.Printf("Dragging from (%f, %f) to (%f, %f)", x1, y1, x2, y2)
	var elementStart string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("document.elementFromPoint(%f, %f)?.tagName || 'NONE'", x1, y1), &elementStart)); err == nil {
		log.Printf("Element at drag start: %s", elementStart)
	}
	var elementEnd string
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("document.elementFromPoint(%f, %f)?.tagName || 'NONE'", x2, y2), &elementEnd)); err == nil {
		log.Printf("Element at drag end: %s", elementEnd)
	}
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := input.DispatchMouseEvent(input.MouseMoved, x1, y1).Do(ctx); err != nil {
				return err
			}
			if err := input.DispatchMouseEvent(input.MousePressed, x1, y1).WithButton("left").WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := input.DispatchMouseEvent(input.MouseMoved, x2, y2).WithButtons(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := input.DispatchMouseEvent(input.MouseReleased, x2, y2).WithButton("left").WithClickCount(1).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
	)
	_ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
	return "dragged", err
}

func handleHover(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	x, y, err := getCoords(args, width, height)
	if err != nil {
		return nil, err
	}
	lastMouseX = x
	lastMouseY = y
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
