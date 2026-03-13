package viewer

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
	"github.com/ghchinoy/riptide/pkg/utils"
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

// Start initializes and runs the session viewer web server.
// If port is empty, it defaults to ":8083".
func Start(port string) error {
	if port == "" {
		port = ":8083"
	}
	
	utils.LoadConfig()
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	workDir, _ := os.Getwd()
	sessionsBaseDir := filepath.Join(workDir, "sessions")

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/sessions", listSessions)
		r.Get("/sessions/{id}", getSession)

		// Serve sessions content (logs, screenshots) under API
		r.Handle("/sessions/*", http.StripPrefix("/api/v1/sessions/", http.FileServer(http.Dir(sessionsBaseDir))))
	})

	// Serve sessions at root too for direct asset access if needed by frontend
	r.Handle("/sessions/*", http.StripPrefix("/sessions/", http.FileServer(http.Dir(sessionsBaseDir))))
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

	log.Printf("Session Viewer backend listening on %s", port)
	return http.ListenAndServe(port, r)
}

func listSessions(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir("sessions")
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var sessions []Session

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		id := entry.Name()
		logPath := filepath.Join("sessions", id, "session.log")

		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			continue
		}

		info, _ := entry.Info()

		// Peek at the log to get the prompt
		prompt, status := peekMetadata(logPath)

		sessions = append(sessions, Session{
			ID:        id,
			Timestamp: info.ModTime(),
			Prompt:    prompt,
			LogPath:   "session.log",
			Status:    status,
		})
	}
	// Sort by newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

func getSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logPath := filepath.Join("sessions", id, "session.log")

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
	defer func() { _ = file.Close() }()

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
				Screenshot: fmt.Sprintf("screenshots/turn_%d_post.png", idx),
				FullPage:   fmt.Sprintf("screenshots/turn_%d_full.png", idx),
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
	_ = json.NewEncoder(w).Encode(session)
}

func peekMetadata(path string) (string, string) {
	file, err := os.Open(path)
	if err != nil {
		return "", "unknown"
	}
	defer func() { _ = file.Close() }()

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