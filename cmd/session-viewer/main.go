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

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/sessions", listSessions)
		r.Get("/sessions/{id}", getSession)
	})

	// Serve screenshots
	workDir, _ := os.Getwd()
	screenshotDir := http.Dir(filepath.Join(workDir, "screenshots"))
	r.Handle("/screenshots/*", http.StripPrefix("/screenshots/", http.FileServer(screenshotDir)))

	// Serve Lit Frontend
	frontendDir := http.Dir(filepath.Join(workDir, "frontend/dist"))
	fileServer(r, "/", frontendDir)

	// SPA Routing fallback
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(workDir, "frontend/dist/index.html"))
	})

	log.Println("Session Viewer backend listening on :8083")
	http.ListenAndServe(":8083", r)
}

// fileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
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
		prompt := peekPrompt(filepath.Join("logs", f.Name()))

		sessions = append(sessions, Session{
			ID:        id,
			Timestamp: info.ModTime(),
			Prompt:    prompt,
			LogPath:   f.Name(),
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
			session.Prompt = m[1]
		}

		if m := turnRe.FindStringSubmatch(line); len(m) > 1 {
			idx, _ := strconv_atoi(m[1])
			if currentTurn != nil {
				turns = append(turns, *currentTurn)
			}
			currentTurn = &Turn{
				Index:      idx,
				Screenshot: fmt.Sprintf("%s/turn_%d_post.png", id, idx),
				FullPage:   fmt.Sprintf("%s/turn_%d_full.png", id, idx),
			}
		}

		if currentTurn != nil {
			if m := thinkingRe.FindStringSubmatch(line); len(m) > 1 {
				currentTurn.Thinking = append(currentTurn.Thinking, m[1])
			}
			if m := actionRe.FindStringSubmatch(line); len(m) > 1 {
				currentTurn.Action = m[1]
			}
		}
	}
	if currentTurn != nil {
		turns = append(turns, *currentTurn)
	}
	session.Turns = turns

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func peekPrompt(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	promptRe := regexp.MustCompile(`\[log\] Prompt: (.+)`)
	for scanner.Scan() {
		if m := promptRe.FindStringSubmatch(scanner.Text()); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

// Minimal helper to avoid importing strconv for one func
func strconv_atoi(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
