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

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Session struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Prompt    string    `json:"prompt"`
	LogPath   string    `json:"log_path"`
	Status    string    `json:"status"` // "active" or "finished"
	Turns     []Turn    `json:"turns,omitempty"`
}

type Turn struct {
	Index      int      `json:"index"`
	Thinking   []string `json:"thinking"`
	Action     string   `json:"action"`
	Screenshot string   `json:"screenshot"`
	FullPage   string   `json:"full_page"`
}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	workDir, _ := os.Getwd()
	screenshotDir := http.Dir(filepath.Join(workDir, "screenshots"))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/sessions", listSessions)
		r.Get("/sessions/{id}", getSession)

		// Serve screenshots under API as well for the frontend
		r.Handle("/screenshots/*", http.StripPrefix("/api/v1/screenshots/", http.FileServer(screenshotDir)))
	})

	// Serve screenshots at root too
	r.Handle("/screenshots/*", http.StripPrefix("/screenshots/", http.FileServer(screenshotDir)))

	// Serve Static Assets
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(workDir, "frontend/dist/assets")))))

	// SPA Routing fallback
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(workDir, "frontend/dist/index.html"))
	})

	log.Println("Session Viewer backend listening on :8083")
	err := http.ListenAndServe(":8083", r)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func listSessions(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir("logs")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var sessions []Session
	re := regexp.MustCompile(`session_(.+)\.log`)

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".log") {
			continue
		}

		matches := re.FindStringSubmatch(f.Name())
		if len(matches) < 2 {
			continue
		}
		id := matches[1]

		info, _ := f.Info()
		
		// Peek at the log to get the prompt
		prompt, status := peekMetadata(filepath.Join("logs", f.Name()))

		sessions = append(sessions, Session{
			ID:        id,
			Timestamp: info.ModTime(),
			Prompt:    prompt,
			LogPath:   f.Name(),
			Status:    status,
		})
	}

	// Sort by newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func getSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logPath := filepath.Join("logs", fmt.Sprintf("session_%s.log", id))

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	session := Session{
		ID: id,
	}

	file, err := os.Open(logPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	var turns []Turn
	var currentTurn *Turn
	
	scanner := bufio.NewScanner(file)
	turnRe := regexp.MustCompile(`\[status\] Turn (\d+)/`)
	promptRe := regexp.MustCompile(`\[log\] Prompt: (.+)`)
	thinkingRe := regexp.MustCompile(`\[thinking\] (.+)`)
	actionRe := regexp.MustCompile(`\[action\] Tool Call: (.+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if m := promptRe.FindStringSubmatch(line); len(m) > 1 {
			session.Prompt = strings.TrimSuffix(m[1], " <nil>")
		}

		if m := turnRe.FindStringSubmatch(line); len(m) > 1 {
			idx, _ := strconv_atoi(m[1])
			if currentTurn != nil {
				turns = append(turns, *currentTurn)
			}
			currentTurn = &Turn{
				Index:      idx,
				Thinking:   []string{},
				Screenshot: fmt.Sprintf("%s/turn_%d_post.png", id, idx),
				FullPage:   fmt.Sprintf("%s/turn_%d_full.png", id, idx),
			}
		}

		if currentTurn != nil {
			if m := thinkingRe.FindStringSubmatch(line); len(m) > 1 {
				currentTurn.Thinking = append(currentTurn.Thinking, strings.TrimSuffix(m[1], " <nil>"))
			}
			if m := actionRe.FindStringSubmatch(line); len(m) > 1 {
				currentTurn.Action = strings.TrimSuffix(m[1], " <nil>")
			}
		}
	}
	if currentTurn != nil {
		turns = append(turns, *currentTurn)
	}
	session.Turns = turns
	log.Printf("Returning session %s with %d turns", session.ID, len(session.Turns))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func peekMetadata(path string) (string, string) {
	file, err := os.Open(path)
	if err != nil {
		return "", "unknown"
	}
	defer file.Close()

	prompt := ""
	status := "active"

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "[log] Prompt:") {
			if m := regexp.MustCompile(`\[log\] Prompt: (.+)`).FindStringSubmatch(line); len(m) > 1 {
				prompt = strings.TrimSuffix(m[1], " <nil>")
			}
		}
		if strings.Contains(line, "Session Finished.") {
			status = "finished"
		}
	}
	return prompt, status
}

// Minimal helper to avoid importing strconv for one func
func strconv_atoi(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
