package computer

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"google.golang.org/genai"
)

// This test requires GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION to be set.
// It runs a real GenAI session against the local test server.
func TestScenario_Lights(t *testing.T) {
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if projectId == "" || location == "" {
		t.Skip("Skipping integration test: GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION not set")
	}

	// 1. Start Test Server
	mux := http.NewServeMux()
	// Reuse the HTML from testserver/main.go or similar
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body><h1>Dashboard</h1><div id="loading">Loading...</div><div id="tiles" style="display:none;"><div id="great-room">Great Room Lights <input type="range" value="0" oninput="this.nextSibling.innerText=this.value"><span>0</span></div><div style="height:1000px"></div><div id="main-hallway">Main Bedroom Hallway lights <input type="range" value="0" oninput="this.nextSibling.innerText=this.value"><span>0</span></div></div><script>setTimeout(() => { document.getElementById('loading').style.display='none'; document.getElementById('tiles').style.display='block'; }, 2000);</script></body></html>`)
	})
	srv := &http.Server{Addr: ":8081", Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// 2. Setup Client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectId,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	                        // 3. Run Agent

	                        sessionID := "test-integration-lights"

	                        prompt := "Go to http://localhost:8081/, wait for tiles to appear, and set Great Room Lights and Main Bedroom Hallway lights to 15%. You will need to scroll."

	                        ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	                

	                        err = Run(ctx, client, "sessions", sessionID, prompt, false, false, ua, true, nil, nil, 10, 3, "default")

	                        if err != nil {

	                

	        

	                t.Errorf("Scenario failed: %v", err)

	        }

	}

	
