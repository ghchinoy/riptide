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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"google.golang.org/genai"
)

func TestExecute_Denormalize(t *testing.T) {
	tests := []struct {
		val      interface{}
		max      int
		expected float64
	}{
		{500.0, 1000, 500.0},
		{500.0, 2000, 1000.0},
		{0.0, 1000, 0.0},
		{1000.0, 1000, 1000.0},
		{500, 1000, 500.0}, // int test
	}

	for _, tt := range tests {
		got := denormalize(tt.val, tt.max)
		if got != tt.expected {
			t.Errorf("denormalize(%v, %v) = %v; want %v", tt.val, tt.max, got, tt.expected)
		}
	}
}

func TestExecute_GetCoords(t *testing.T) {
	width, height := 1000, 1000

	t.Run("coordinate_array", func(t *testing.T) {
		args := map[string]interface{}{"coordinate": []interface{}{500.0, 500.0}}
		x, y, err := getCoords(args, width, height)
		if err != nil || x != 500 || y != 500 {
			t.Errorf("getCoords failed: %v, %v, %v", x, y, err)
		}
	})

	t.Run("x_y_fields", func(t *testing.T) {
		args := map[string]interface{}{"x": 250.0, "y": 750.0}
		x, y, err := getCoords(args, width, height)
		if err != nil || x != 250 || y != 750 {
			t.Errorf("getCoords failed: %v, %v, %v", x, y, err)
		}
	})
}

// TestExecutor_Integration runs the executor against a real headless browser
func TestExecutor_Integration(t *testing.T) {
	var mu sync.Mutex
	clicks := make(map[string]int)
	lastType := ""

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `
				<html><body>
					<button id="btn" style="position:absolute;top:0;left:0;width:100px;height:50px" onclick="fetch('/click?id=btn')">Click Me</button>
					<input id="input" style="position:absolute;top:100px;left:0" onchange="fetch('/type?val='+this.value)">
					<button id="hoverbtn" style="position:absolute;top:200px;left:0;width:100px;height:50px" onmouseover="fetch('/hover?id=hoverbtn')">Hover Me</button>
					
					<button id="rightbtn" style="position:absolute;top:300px;left:0;width:100px;height:50px" oncontextmenu="event.preventDefault(); fetch('/rightclick?id=rightbtn')">Right Click</button>
					<button id="midbtn" style="position:absolute;top:400px;left:0;width:100px;height:50px" onauxclick="if(event.button === 1) fetch('/midclick?id=midbtn')">Mid Click</button>
					<button id="dblbtn" style="position:absolute;top:500px;left:0;width:100px;height:50px" ondblclick="fetch('/dblclick?id=dblbtn')">Double Click</button>
					<div id="movezone" style="position:absolute;top:600px;left:0;width:100px;height:50px;background:red" onmousemove="fetch('/mousemove?id=movezone')">Move Zone</div>
				</body></html>`)
		case "/click":
			clicks[r.URL.Query().Get("id")]++
		case "/hover":
			clicks[r.URL.Query().Get("id")]++
		case "/rightclick":
			clicks[r.URL.Query().Get("id")]++
		case "/midclick":
			clicks[r.URL.Query().Get("id")]++
		case "/dblclick":
			clicks[r.URL.Query().Get("id")]++
		case "/mousemove":
			clicks[r.URL.Query().Get("id")]++
		case "/type":
			lastType = r.URL.Query().Get("val")
		}
	}))
	defer ts.Close()

	// Browser Setup
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.WindowSize(1280, 1024))
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	runTest := func(t *testing.T, name string, testFn func(context.Context)) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := chromedp.NewContext(allocCtx)
			defer cancel()
			
			// Initial navigation for every test to be sure
			if err := chromedp.Run(ctx, chromedp.Navigate(ts.URL), chromedp.WaitReady("body")); err != nil {
				t.Fatalf("Failed to navigate: %v", err)
			}
			testFn(ctx)
		})
	}

	runTest(t, "Click", func(ctx context.Context) {
		// btn is at 0,0 to 100,50. Center is 50, 25.
		nx := (50.0 / 1280.0) * 1000.0
		ny := (25.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "mouse_click",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("Click failed: %v", err)
		}
		
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if clicks["btn"] == 0 {
			t.Errorf("Click was not registered")
		}
	})

	runTest(t, "Type", func(ctx context.Context) {
		// input is at 0, 100.
		nx := (50.0 / 1280.0) * 1000.0
		ny := (110.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "type",
			Args: map[string]interface{}{
				"x":    nx,
				"y":    ny,
				"text": "Riptide",
			},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("Type failed: %v", err)
		}

		// Ensure blur for change event
		_ = chromedp.Run(ctx, chromedp.KeyEvent("\r"))
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		if lastType != "Riptide" {
			t.Errorf("Type failed, got: %q", lastType)
		}
	})

	runTest(t, "Key", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (110.0 / 1024.0) * 1000.0
		// Click to focus
		_, _ = Execute(ctx, &genai.FunctionCall{
			Name: "mouse_click",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}, 1280, 1024)

		time.Sleep(100 * time.Millisecond)

		// Send keys
		_, err := Execute(ctx, &genai.FunctionCall{
			Name: "key",
			Args: map[string]interface{}{"text": "Hello"},
		}, 1280, 1024)
		if err != nil {
			t.Fatalf("Key failed: %v", err)
		}

		_ = chromedp.Run(ctx, chromedp.KeyEvent("\r"))
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		if lastType != "Hello" {
			t.Errorf("Key failed, got: %q", lastType)
		}
	})

	runTest(t, "Hover", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (225.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "hover",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("Hover failed: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if clicks["hoverbtn"] == 0 {
			t.Errorf("Hover was not registered")
		}
	})

	runTest(t, "RightClick", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (325.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "right_click",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("RightClick failed: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if clicks["rightbtn"] == 0 {
			t.Errorf("Right click was not registered")
		}
	})

	runTest(t, "MiddleClick", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (425.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "middle_click",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("MiddleClick failed: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if clicks["midbtn"] == 0 {
			t.Errorf("Middle click was not registered")
		}
	})

	runTest(t, "DoubleClick", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (525.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "double_click",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("DoubleClick failed: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if clicks["dblbtn"] == 0 {
			t.Errorf("Double click was not registered")
		}
	})

	runTest(t, "MouseMove", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (625.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "mouse_move",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("MouseMove failed: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if clicks["movezone"] == 0 {
			t.Errorf("Mouse move was not registered")
		}
	})

	runTest(t, "GetComputedStyle", func(ctx context.Context) {
		nx := (50.0 / 1280.0) * 1000.0
		ny := (25.0 / 1024.0) * 1000.0
		call := &genai.FunctionCall{
			Name: "get_computed_style",
			Args: map[string]interface{}{"x": nx, "y": ny},
		}
		res, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("GetComputedStyle failed: %v", err)
		}
		
		out, ok := res["output"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map output, got %T", res["output"])
		}
		if out["tagName"] != "BUTTON" {
			t.Errorf("Expected BUTTON tag, got %v", out["tagName"])
		}
	})

	runTest(t, "GetPageLayout", func(ctx context.Context) {
		call := &genai.FunctionCall{
			Name: "get_page_layout",
			Args: map[string]interface{}{},
		}
		res, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("GetPageLayout failed: %v", err)
		}
		out, ok := res["output"].([]interface{})
		if !ok || len(out) == 0 {
			t.Fatalf("Expected slice output with elements, got %T", res["output"])
		}
	})

	runTest(t, "Search", func(ctx context.Context) {
		call := &genai.FunctionCall{
			Name: "search",
			Args: map[string]interface{}{"url": ts.URL}, // Since search maps to navigate, it needs a URL argument now
		}
		_, err := Execute(ctx, call, 1280, 1024)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
	})
}

func TestStatusReporting(t *testing.T) {
	// Verify that Execute correctly returns the current URL
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "OK")
	}))
	defer ts.Close()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	call := &genai.FunctionCall{
		Name: "navigate",
		Args: map[string]interface{}{"url": ts.URL},
	}
	res, err := Execute(ctx, call, 1280, 1024)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if res["url"] != ts.URL+"/" {
		t.Errorf("Expected URL %s, got %s", ts.URL+"/", res["url"])
	}
}
