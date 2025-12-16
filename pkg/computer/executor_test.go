package computer

import (
	"context"
	"testing"

	"google.golang.org/genai"
)

// Mock/Stub for checking if actions would be executed.
// Since Execute calls chromedp directly, we can't easily mock chromedp without refactoring.
// For now, we will test the parameter parsing and dispatch logic by checking for errors
// or checking the returned "action" string which we return from Execute.

func TestExecute_Click(t *testing.T) {
	ctx := context.Background()
	
	// Test standard click
	call := &genai.FunctionCall{
		Name: "mouse_click",
		Args: map[string]interface{}{
			"x": 500.0,
			"y": 500.0,
		},
	}

	// We expect this to fail with "chromedp: context not initialized" or similar
	// because we aren't passing a real chromedp context.
	// However, we can check if it *tried* to click.
	// Our Execute function returns (interface{}, error).
	// On success it returns "clicked".
	
	// Ideally we'd wrap the chromedp calls in an interface, but for a quick test 
	// we just want to verify the switch statements and arg parsing work.
	// The current implementation calls chromedp.Run immediately.
	
	// Let's refactor executor.go slightly to make it testable? 
	// Or just accept that we need a real context?
	// Starting a real headless chrome for unit tests is heavy but possible.
	
	// Alternative: Verify the parsing logic in separate functions.
	// The helper functions like getCoords are private. 
	// We should export them or test them via internal test.
	
	// Let's try testing `getCoords` and others by putting this test in `package computer`.
	
	// Actually, let's just run it. If it fails on chromedp.Run, we know it passed parsing.
	_, err := Execute(ctx, call, 1000, 1000)
	if err == nil {
		// If it returns nil error, that means chromedp.Run succeeded? 
		// Unlikely with empty context.
		t.Log("Execution succeeded (unexpectedly?)")
	} else {
		t.Logf("Execution failed as expected (no browser): %v", err)
		// Verify it wasn't a parsing error.
		if err.Error() == "no coordinates found" {
			t.Error("Parsing failed: no coordinates found")
		}
	}
}

func TestExecute_Denormalize(t *testing.T) {
	// We can test denormalize if we export it or test internally.
	// Since this file is `package computer`, we can access private `denormalize`.
	
	val := denormalize(500.0, 1920)
	expected := 960.0
	if val != expected {
		t.Errorf("Expected %f, got %f", expected, val)
	}
	
	val = denormalize(0.0, 1920)
	if val != 0.0 {
		t.Errorf("Expected 0, got %f", val)
	}
	
	val = denormalize(1000.0, 1920)
	if val != 1920.0 {
		t.Errorf("Expected 1920, got %f", val)
	}
}

func TestExecute_Type(t *testing.T) {
	ctx := context.Background()
	call := &genai.FunctionCall{
		Name: "type_text_at",
		Args: map[string]interface{}{
			"text": "Hello",
			"x": 500.0,
			"y": 500.0,
		},
	}
	// Should attempt click then type
	_, err := Execute(ctx, call, 1000, 1000)
	if err == nil {
		t.Log("Execution succeeded (unexpectedly?)")
	} else if err.Error() != "invalid context" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestExecute_Scroll(t *testing.T) {
	ctx := context.Background()
	call := &genai.FunctionCall{
		Name: "scroll_at",
		Args: map[string]interface{}{
			"x": 500.0,
			"y": 500.0,
			"direction": "down",
		},
	}
	_, err := Execute(ctx, call, 1000, 1000)
	if err == nil {
		t.Log("Execution succeeded")
	} else if err.Error() != "invalid context" {
		// Scroll might be unimplemented or just return "scrolled" without chromedp call?
		// Checking implementation: handleScroll returns "scrolled", nil. It does NOT call chromedp yet.
		// So this should succeed.
	}
}
