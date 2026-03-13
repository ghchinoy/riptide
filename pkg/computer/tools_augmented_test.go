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