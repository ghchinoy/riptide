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
	"google.golang.org/genai"
)

func init() {
	// Register hold_key skill
	RegisterCustomSkill(&CustomSkill{
		Declaration: &genai.FunctionDeclaration{
			Name:        "hold_key",
			Description: "Press and hold a keyboard key for a specified duration in milliseconds (e.g. 500ms). Useful for continuous movement or holding direction keys in web games.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"key": {
						Type:        genai.TypeString,
						Description: "The key to hold (e.g. 'ArrowRight', 'ArrowDown', 'Space', 'Enter').",
					},
					"duration_ms": {
						Type:        genai.TypeInteger,
						Description: "Duration to hold the key in milliseconds (default: 300, min: 50, max: 3000).",
					},
				},
				Required: []string{"key"},
			},
		},
		Handler: handleHoldKey,
	})

	// Register press_keys skill
	RegisterCustomSkill(&CustomSkill{
		Declaration: &genai.FunctionDeclaration{
			Name:        "press_keys",
			Description: "Press a sequence of keyboard keys in order within a single turn with a small delay between presses. Useful for executing multi-step movement along corridors in web games.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"keys": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeString,
						},
						Description: "Ordered list of keys to press in sequence (e.g. ['ArrowDown', 'ArrowRight', 'ArrowDown']).",
					},
					"interval_ms": {
						Type:        genai.TypeInteger,
						Description: "Delay between key presses in milliseconds (default: 100, min: 20, max: 1000).",
					},
				},
				Required: []string{"keys"},
			},
		},
		Handler: handlePressKeys,
	})
}

func handleHoldKey(ctx context.Context, args map[string]interface{}, _, _ int) (interface{}, error) {
	rawKey := resolveKeyArg(args)
	if rawKey == "" {
		return nil, fmt.Errorf("no key provided for hold_key")
	}

	durationMs := 300
	if d, ok := args["duration_ms"].(float64); ok && d > 0 {
		durationMs = int(d)
	} else if d, ok := args["duration_ms"].(int); ok && d > 0 {
		durationMs = d
	}

	// Clamp duration
	if durationMs < 50 {
		durationMs = 50
	}
	if durationMs > 3000 {
		durationMs = 3000
	}

	log.Printf("Holding key %s for %d ms", rawKey, durationMs)

	if err := dispatchFullKeyEvent(ctx, rawKey, input.KeyDown); err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(durationMs) * time.Millisecond)

	err := dispatchFullKeyEvent(ctx, rawKey, input.KeyUp)
	return fmt.Sprintf("held_%s_%dms", rawKey, durationMs), err
}

func handlePressKeys(ctx context.Context, args map[string]interface{}, w, h int) (interface{}, error) {
	var keys []string
	if kList, ok := args["keys"].([]interface{}); ok {
		for _, item := range kList {
			if s, ok := item.(string); ok && s != "" {
				keys = append(keys, s)
			}
		}
	} else if kList, ok := args["keys"].([]string); ok {
		keys = kList
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys provided for press_keys")
	}

	// Limit to max 10 keys per batch
	if len(keys) > 10 {
		keys = keys[:10]
	}

	intervalMs := 100
	if val, ok := args["interval_ms"].(float64); ok && val >= 0 {
		intervalMs = int(val)
	} else if val, ok := args["interval_ms"].(int); ok && val >= 0 {
		intervalMs = val
	}

	// Clamp interval
	if intervalMs < 20 {
		intervalMs = 20
	}
	if intervalMs > 1000 {
		intervalMs = 1000
	}

	log.Printf("Pressing key sequence %v with %d ms interval", keys, intervalMs)

	for i, key := range keys {
		if _, err := handleKey(ctx, map[string]interface{}{"key": key}, w, h); err != nil {
			return nil, fmt.Errorf("error pressing key %s at index %d: %w", key, i, err)
		}
		if i < len(keys)-1 {
			time.Sleep(time.Duration(intervalMs) * time.Millisecond)
		}
	}

	return fmt.Sprintf("pressed_%d_keys", len(keys)), nil
}
