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
		// Only include nodes that are likely interactive or provide content
		if role == "StaticText" || role == "generic" || role == "LineBreak" {
			continue
		}

		sNode := SimpleAXNode{
			Role:  role,
			State: make(map[string]string),
		}

		if node.Name != nil {
			sNode.Name = fmt.Sprintf("%v", node.Name.Value)
		}

		if node.Value != nil {
			sNode.Value = fmt.Sprintf("%v", node.Value.Value)
		}

		// Extract interesting states
		for _, prop := range node.Properties {
			if prop.Value == nil {
				continue
			}
			val := fmt.Sprintf("%v", prop.Value.Value)
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