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

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

func handleGetAccessibilityTree(ctx context.Context, args map[string]interface{}, width, height int) (interface{}, error) {
	log.Printf("Getting accessibility tree...")
	var nodes []*accessibility.Node
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		nodes, err = accessibility.GetFullAXTree().Do(ctx)
		return err
	}))
	if err != nil {
		return nil, err
	}

	return simplifyAXTree(nodes), nil
}

type SimpleAXNode struct {
	Role  string            `json:"role,omitempty"`
	Name  string            `json:"name,omitempty"`
	Value string            `json:"value,omitempty"`
	State map[string]string `json:"state,omitempty"`
}

func simplifyAXTree(nodes []*accessibility.Node) []SimpleAXNode {
	var results []SimpleAXNode
	for _, node := range nodes {
		if node.Role == nil {
			continue
		}

		role := fmt.Sprintf("%v", node.Role.Value)
		// Strip quotes if present (jsontext.Value formats strings with quotes)
		if len(role) >= 2 && role[0] == '"' && role[len(role)-1] == '"' {
			role = role[1 : len(role)-1]
		}
		
		// Only include nodes that are likely interactive or provide content
		if role == "StaticText" || role == "generic" || role == "LineBreak" {
			continue
		}

		sNode := SimpleAXNode{
			Role:  role,
			State: make(map[string]string),
		}

		if node.Name != nil {
			nameVal := fmt.Sprintf("%v", node.Name.Value)
			if len(nameVal) >= 2 && nameVal[0] == '"' && nameVal[len(nameVal)-1] == '"' {
				nameVal = nameVal[1 : len(nameVal)-1]
			}
			sNode.Name = nameVal
		}

		if node.Value != nil {
			valStr := fmt.Sprintf("%v", node.Value.Value)
			if len(valStr) >= 2 && valStr[0] == '"' && valStr[len(valStr)-1] == '"' {
				valStr = valStr[1 : len(valStr)-1]
			}
			sNode.Value = valStr
		}

		// Extract interesting states
		for _, prop := range node.Properties {
			if prop.Value == nil {
				continue
			}
			val := fmt.Sprintf("%v", prop.Value.Value)
			if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
				val = val[1 : len(val)-1]
			}
			if val == "false" || val == "" {
				continue
			}
			sNode.State[string(prop.Name)] = val
		}

		if sNode.Name == "" && sNode.Value == "" && len(sNode.State) == 0 {
			continue
		}

		results = append(results, sNode)
	}
	return results
}
