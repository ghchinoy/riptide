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

// Package computer — regression test for the empty-screenshot guard.
//
// Discovered during live Tier 1 validation: chromedp occasionally returns a
// zero-byte screenshot buffer on a freshly-created target. Appending an empty
// InlineData blob caused the Vertex API to reject the entire request with:
//   400: parts[1].data: required oneof field 'data' must have one initialized field
// The guard omits empty blobs from both the initial message and FunctionResponse.
package computer

import (
	"testing"

	"google.golang.org/genai"
)

// buildInitialContent replicates the initial-message construction logic:
// a text part, plus a screenshot InlineData part only if the buffer is non-empty.
func buildInitialContent(prompt string, buf []byte) *genai.Content {
	c := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: prompt}},
	}
	if len(buf) > 0 {
		c.Parts = append(c.Parts, &genai.Part{
			InlineData: &genai.Blob{MIMEType: "image/png", Data: buf},
		})
	}
	return c
}

// buildFunctionResponse replicates the post-action FunctionResponse logic:
// screenshot attached only when non-empty.
func buildFunctionResponse(name string, result map[string]interface{}, buf []byte) *genai.Part {
	p := &genai.Part{
		FunctionResponse: &genai.FunctionResponse{Name: name, Response: result},
	}
	if len(buf) > 0 {
		p.FunctionResponse.Parts = []*genai.FunctionResponsePart{
			{InlineData: &genai.FunctionResponseBlob{MIMEType: "image/png", Data: buf}},
		}
	}
	return p
}

// hasEmptyInlineData reports whether any part carries an InlineData blob with
// zero-length data — the exact condition the Vertex API rejects with 400.
func hasEmptyInlineData(c *genai.Content) bool {
	for _, p := range c.Parts {
		if p.InlineData != nil && len(p.InlineData.Data) == 0 {
			return true
		}
		if p.FunctionResponse != nil {
			for _, frp := range p.FunctionResponse.Parts {
				if frp.InlineData != nil && len(frp.InlineData.Data) == 0 {
					return true
				}
			}
		}
	}
	return false
}

func TestEmptyScreenshot_InitialMessageOmitsEmptyBlob(t *testing.T) {
	c := buildInitialContent("do a task", []byte{})
	if hasEmptyInlineData(c) {
		t.Error("empty initial screenshot must NOT produce an empty InlineData blob")
	}
	// Only the text part should remain.
	if len(c.Parts) != 1 {
		t.Errorf("expected 1 part (text only) for empty screenshot, got %d", len(c.Parts))
	}
	if c.Parts[0].Text == "" {
		t.Error("text prompt must be preserved even when screenshot is empty")
	}
}

func TestEmptyScreenshot_InitialMessageKeepsNonEmptyBlob(t *testing.T) {
	c := buildInitialContent("do a task", []byte("realpngbytes"))
	if hasEmptyInlineData(c) {
		t.Error("non-empty screenshot incorrectly flagged as empty")
	}
	if len(c.Parts) != 2 {
		t.Errorf("expected 2 parts (text + screenshot), got %d", len(c.Parts))
	}
}

func TestEmptyScreenshot_FunctionResponseOmitsEmptyBlob(t *testing.T) {
	p := buildFunctionResponse("click", map[string]interface{}{"output": "clicked"}, []byte{})
	c := &genai.Content{Role: "user", Parts: []*genai.Part{p}}
	if hasEmptyInlineData(c) {
		t.Error("empty post-action screenshot must NOT produce an empty InlineData blob")
	}
	// The function response itself (text result) must still be present.
	if p.FunctionResponse == nil || p.FunctionResponse.Response == nil {
		t.Error("function response result must be preserved even without screenshot")
	}
	if len(p.FunctionResponse.Parts) != 0 {
		t.Errorf("expected 0 screenshot parts for empty buffer, got %d", len(p.FunctionResponse.Parts))
	}
}

func TestEmptyScreenshot_FunctionResponseKeepsNonEmptyBlob(t *testing.T) {
	p := buildFunctionResponse("click", map[string]interface{}{"output": "clicked"}, []byte("realpng"))
	if len(p.FunctionResponse.Parts) != 1 {
		t.Errorf("expected 1 screenshot part for non-empty buffer, got %d", len(p.FunctionResponse.Parts))
	}
}
