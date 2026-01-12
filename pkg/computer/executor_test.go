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
			fmt.Fprint(w, `
				<html><body>
					<button id="btn" style="position:absolute;top:0;left:0;width:100px;height:50px" onclick="fetch('/click?id=btn')">Click Me</button>
					<input id="input" style="position:absolute;top:100px;left:0" onchange="fetch('/type?val='+this.value)">
				</body></html>`)
		case "/click":
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
		chromedp.Run(ctx, chromedp.KeyEvent("\r"))
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		if lastType != "Riptide" {
			t.Errorf("Type failed, got: %q", lastType)
		}
	})
}

func TestStatusReporting(t *testing.T) {
	// Verify that Execute correctly returns the current URL
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
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
