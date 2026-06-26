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

// Package computer — tests for Gemini 3.5 Flash new function call names.
// Validates that every name in PREDEFINED_COMPUTER_USE_FUNCTIONS from the
// reference implementation is handled by Riptide's executor without falling
// through to the hallucination interceptor.
package computer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"google.golang.org/genai"
)

// allGemini35Names lists every predefined function name emitted by
// gemini-3.5-flash, sourced from sources/computer-use-preview/agent.py.
// All of these must be registered in the tool registry.
var allGemini35Names = []string{
	"click",
	"double_click",
	"triple_click",
	"middle_click",
	"right_click",
	"mouse_down",
	"mouse_up",
	"move",
	"type",
	"drag_and_drop",
	"wait",
	"press_key",
	"key_down",
	"key_up",
	"hotkey",
	"take_screenshot",
	"scroll",
	"go_back",
	"navigate",
	"go_forward",
}

// TestAllGemini35NamesRegistered verifies that every 3.5 function name is
// known to the registry — i.e., none would trigger the hallucination interceptor.
func TestAllGemini35NamesRegistered(t *testing.T) {
	for _, name := range allGemini35Names {
		t.Run(name, func(t *testing.T) {
			if !IsToolKnown(name) {
				t.Errorf("tool %q not registered — would be caught by hallucination interceptor", name)
			}
		})
	}
}

// TestResolveKeyArg verifies all arg key variants are extracted correctly.
func TestResolveKeyArg(t *testing.T) {
	tests := []struct {
		args map[string]interface{}
		want string
	}{
		{map[string]interface{}{"key": "Enter"}, "Enter"},
		{map[string]interface{}{"text": "ctrl+c"}, "ctrl+c"},
		{map[string]interface{}{"value": "Escape"}, "Escape"},
		{map[string]interface{}{"keys": "Meta+a"}, "Meta+a"},
		{map[string]interface{}{}, ""},
	}
	for _, tt := range tests {
		got := resolveKeyArg(tt.args)
		if got != tt.want {
			t.Errorf("resolveKeyArg(%v) = %q; want %q", tt.args, got, tt.want)
		}
	}
}

// TestScrollMagnitude verifies the new 3.5 magnitude arg is applied correctly.
func TestScrollMagnitude(t *testing.T) {
	// direction=down, magnitude=400 at 1024px height → dy = 400/1000*1024 ≈ 409.6
	// We just test the math path doesn't crash and returns "scrolled".
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body style="height:5000px">scroll test</body></html>`)
	}))
	defer ts.Close()

	if err := chromedp.Run(ctx, chromedp.Navigate(ts.URL), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	call := &genai.FunctionCall{
		Name: "scroll",
		Args: map[string]interface{}{
			"direction": "down",
			"magnitude": 400.0,
		},
	}
	res, err := Execute(ctx, call, 1280, 1024)
	if err != nil {
		t.Fatalf("scroll with magnitude: %v", err)
	}
	if res["output"] != "scrolled" {
		t.Errorf("expected output='scrolled', got %v", res["output"])
	}
}

// TestGemini35Tools_Integration exercises the newly added 3.5 actions against
// a real headless browser to ensure they don't crash or misbehave.
func TestGemini35Tools_Integration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `
<html><body>
  <button id="btn" style="position:absolute;top:50px;left:50px;width:100px;height:40px">Click</button>
  <input id="inp" style="position:absolute;top:150px;left:50px" value="hello">
  <div style="height:3000px">tall page</div>
</body></html>`)
	}))
	defer ts.Close()

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.WindowSize(1280, 1024))
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	run := func(t *testing.T, name string, fn func(context.Context)) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := chromedp.NewContext(allocCtx)
			defer cancel()
			if err := chromedp.Run(ctx,
				chromedp.Navigate(ts.URL),
				chromedp.WaitReady("body"),
			); err != nil {
				t.Fatalf("navigate: %v", err)
			}
			fn(ctx)
		})
	}

	btnX := (100.0 / 1280.0) * 1000.0 // ~78
	btnY := (70.0 / 1024.0) * 1000.0  // ~68

	run(t, "triple_click", func(ctx context.Context) {
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "triple_click",
			Args: map[string]interface{}{"x": btnX, "y": btnY},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("triple_click: %v", err)
		}
	})

	run(t, "move", func(ctx context.Context) {
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "move",
			Args: map[string]interface{}{"x": btnX, "y": btnY},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("move: %v", err)
		}
	})

	run(t, "mouse_down_up", func(ctx context.Context) {
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "mouse_down",
			Args: map[string]interface{}{"x": btnX, "y": btnY},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("mouse_down: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		_, err = Execute(ctx, &genai.FunctionCall{
			Name: "mouse_up",
			Args: map[string]interface{}{"x": btnX, "y": btnY},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("mouse_up: %v", err)
		}
	})

	run(t, "hotkey", func(ctx context.Context) {
		// Focus the input first, then send hotkey
		_, _ = Execute(ctx, &genai.FunctionCall{
			Name: "click",
			Args: map[string]interface{}{
				"x": (100.0 / 1280.0) * 1000.0,
				"y": (160.0 / 1024.0) * 1000.0,
			},
		}, 1280, 1024)
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "hotkey",
			Args: map[string]interface{}{"keys": "ctrl+a"},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("hotkey: %v", err)
		}
	})

	run(t, "take_screenshot", func(ctx context.Context) {
		res, err := Execute(ctx, &genai.FunctionCall{
			Name: "take_screenshot",
			Args: map[string]interface{}{},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("take_screenshot: %v", err)
		}
		if res["output"] != "screenshot_captured_by_harness" {
			t.Errorf("unexpected output: %v", res["output"])
		}
	})

	run(t, "go_forward_no_history", func(ctx context.Context) {
		// go_forward with no forward history — must not hang or crash.
		// Registry coverage is in TestAllGemini35NamesRegistered/go_forward.
		res, err := Execute(ctx, &genai.FunctionCall{
			Name: "go_forward",
			Args: map[string]interface{}{},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("go_forward: %v", err)
		}
		if res["output"] != "navigated_forward" {
			t.Errorf("unexpected output: %v", res["output"])
		}
	})

	run(t, "wait_with_seconds", func(ctx context.Context) {
		start := time.Now()
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "wait",
			Args: map[string]interface{}{"seconds": 0.1},
		}, 1280, 1024)
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("wait: %v", err)
		}
		if elapsed < 90*time.Millisecond {
			t.Errorf("wait too short: %v", elapsed)
		}
	})

	run(t, "scroll_with_magnitude", func(ctx context.Context) {
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "scroll",
			Args: map[string]interface{}{
				"direction": "down",
				"magnitude": 500.0,
			},
		}, 1280, 1024)
		if err != nil {
			t.Errorf("scroll with magnitude: %v", err)
		}
	})
}
