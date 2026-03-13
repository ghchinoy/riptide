package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestPeekMetadata(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "session.log")

	// Test non-existent
	p, s := peekMetadata(logPath)
	if p != "" || s != "unknown" {
		t.Errorf("expected empty/unknown for missing file, got %q, %q", p, s)
	}

	content := `[log] Prompt: This is a test prompt <nil>
[status] Turn 1/10 <nil>
[status] Session Finished. <nil>`
	os.WriteFile(logPath, []byte(content), 0644)

	p, s = peekMetadata(logPath)
	if p != "This is a test prompt" {
		t.Errorf("expected 'This is a test prompt', got %q", p)
	}
	if s != "finished" {
		t.Errorf("expected 'finished', got %q", s)
	}
}

func TestListSessions(t *testing.T) {
	// Setup test directory
	oldWd, _ := os.Getwd()
	tempWd := t.TempDir()
	os.Chdir(tempWd)
	defer os.Chdir(oldWd)

	// Test with no sessions dir
	req := httptest.NewRequest("GET", "/sessions", nil)
	rr := httptest.NewRecorder()
	listSessions(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK when sessions dir is missing, got %v", rr.Code)
	}
	if rr.Body.String() != "[]" {
		t.Errorf("Expected [], got %q", rr.Body.String())
	}

	// Create sessions dir and a mock session
	os.MkdirAll("sessions/sess-123", 0755)
	os.WriteFile("sessions/sess-123/session.log", []byte(`[log] Prompt: Hello <nil>`), 0644)

	rr = httptest.NewRecorder()
	listSessions(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %v", rr.Code)
	}
	var sessions []Session
	if err := json.NewDecoder(rr.Body).Decode(&sessions); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sess-123" || sessions[0].Prompt != "Hello" {
		t.Errorf("Unexpected sessions output: %+v", sessions)
	}
}

func TestGetSession(t *testing.T) {
	// Setup test directory
	oldWd, _ := os.Getwd()
	tempWd := t.TempDir()
	os.Chdir(tempWd)
	defer os.Chdir(oldWd)

	os.MkdirAll("sessions/sess-123", 0755)
	os.WriteFile("sessions/sess-123/session.log", []byte(`[log] Prompt: Test <nil>
[status] Turn 1/10
[thinking] I should do X <nil>
[action] Tool Call: click map[] <nil>`), 0644)

	// Create a dummy index.html for SPA testing
	os.MkdirAll("frontend/dist", 0755)
	os.WriteFile("frontend/dist/index.html", []byte("SPA Fallback"), 0644)

	r := chi.NewRouter()

	// Setup the same routing as main.go
	sessionsBaseDir := filepath.Join(tempWd, "sessions")

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/sessions/{id}", getSession)
	})

	r.Handle("/sessions/*", http.StripPrefix("/sessions/", http.FileServer(http.Dir(sessionsBaseDir))))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(tempWd, "frontend/dist/index.html"))
	})

	// Test valid session
	req := httptest.NewRequest("GET", "/api/v1/sessions/sess-123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %v", rr.Code)
	}

	var session Session
	if err := json.NewDecoder(rr.Body).Decode(&session); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if session.ID != "sess-123" {
		t.Errorf("Expected sess-123, got %s", session.ID)
	}
	if len(session.Turns) != 1 {
		t.Fatalf("Expected 1 turn, got %d", len(session.Turns))
	}
	if session.Turns[0].Action != "click map[]" {
		t.Errorf("Expected action 'click map[]', got %q", session.Turns[0].Action)
	}

	// Test invalid session via API (should 404)
	req = httptest.NewRequest("GET", "/api/v1/sessions/missing", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for API, got %v", rr.Code)
	}

	// Test SPA routing fallback (should 200 and serve index.html)
	req = httptest.NewRequest("GET", "/missing-route", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for SPA fallback, got %v", rr.Code)
	}
	if rr.Body.String() != "SPA Fallback" {
		t.Errorf("Expected 'SPA Fallback', got %q", rr.Body.String())
	}
}