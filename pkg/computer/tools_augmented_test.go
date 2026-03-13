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
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/go-json-experiment/json/jsontext"
)

func TestSimplifyAXTree(t *testing.T) {
	nodes := []*accessibility.Node{
		{
			Role: &accessibility.Value{Value: jsontext.Value(`"StaticText"`)},
			Name: &accessibility.Value{Value: jsontext.Value(`"Ignore me"`)},
		},
		{
			Role: &accessibility.Value{Value: jsontext.Value(`"button"`)},
			Name: &accessibility.Value{Value: jsontext.Value(`"Click Me"`)},
			Properties: []*accessibility.Property{
				{Name: "focusable", Value: &accessibility.Value{Value: jsontext.Value(`"true"`)}},
			},
		},
	}

	res := simplifyAXTree(nodes)
	if len(res) != 1 {
		t.Fatalf("expected 1 node, got %d", len(res))
	}
	if res[0].Name != "Click Me" {
		t.Errorf("expected name 'Click Me', got %s", res[0].Name)
	}
	if res[0].State["focusable"] != "true" {
		t.Errorf("expected focusable state 'true', got %s", res[0].State["focusable"])
	}
}