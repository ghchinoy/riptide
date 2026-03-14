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

	"github.com/ghchinoy/riptide/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
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

// Server represents the viewer HTTP server
type Server struct {
	port string
	hub  *Hub
}

// Global server instance
var defaultServer *Server

func init() {
	defaultServer = &Server{
		port: ":8083",
		hub:  NewHub(),
	}
	go defaultServer.hub.Run()
}

// Start initializes and runs the session viewer web server on the default port.
func Start(port string) error {
	if port != "" {
		defaultServer.port = port
	}
	return defaultServer.Start()
}

// BroadcastEvent allows an external process (like the agent loop) to send an event to all connected UI clients.
func BroadcastEvent(sessionID string, payload []byte) {
	if defaultServer != nil && defaultServer.hub != nil {
		defaultServer.hub.Broadcast <- BroadcastMessage{
			SessionID: sessionID,
			Payload:   payload,
		}
	}
}

// noDirFileSystem wraps http.FileSystem to prevent directory listings
type noDirFileSystem struct {
	fs http.FileSystem
}

func (nfs noDirFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if s.IsDir() {
		// Do not allow directory listings. Return a mocked "not found" error
		// so that the http.FileServer passes control to the NotFound handler.
		f.Close()
		return nil, os.ErrNotExist
	}

	return f, nil
}

// Start initializes and runs the session viewer web server.
func (s *Server) Start() error {
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
		r.Get("/sessions/{id}/stream", s.serveWs) // WebSocket endpoint

		// Serve sessions content (logs, screenshots) under API
		// We allow directory listings here if needed, or we can secure it.
		r.Handle("/sessions/*", http.StripPrefix("/api/v1/sessions/", http.FileServer(http.Dir(sessionsBaseDir))))
	})

	// Serve sessions at root too for direct asset access if needed by frontend
	// Crucially, use noDirFileSystem so that requests for "/sessions/123/" (a directory) 
	// fall through to the SPA NotFound handler instead of returning an HTML folder index.
	r.Handle("/sessions/*", http.StripPrefix("/sessions/", http.FileServer(noDirFileSystem{http.Dir(sessionsBaseDir)})))
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

	log.Printf("Session Viewer backend listening on %s", s.port)
	return http.ListenAndServe(s.port, r)
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
	var sessionLogs []string
	var sessionRaw []map[string]interface{}

	scanner := bufio.NewScanner(file)
	turnRe := regexp.MustCompile(`\[status\] Turn (\d+)/`)
	promptRe := regexp.MustCompile(`\[log\] Prompt: (.+)`)
	thinkingRe := regexp.MustCompile(`\[thinking\] (.+)`)
	actionRe := regexp.MustCompile(`\[action\] Tool Call: (.+)`)
	logRe := regexp.MustCompile(`\[log\] (.+)`)
	rawRe := regexp.MustCompile(`\[raw\] (.+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if m := promptRe.FindStringSubmatch(line); len(m) > 1 {
			session.Prompt = strings.TrimSuffix(m[1], " <nil>")
		}

		if m := logRe.FindStringSubmatch(line); len(m) > 1 {
			// Don't duplicate the prompt log
			if !strings.HasPrefix(m[1], "Prompt:") {
				sessionLogs = append(sessionLogs, strings.TrimSuffix(m[1], " <nil>"))
			}
		}

		if m := rawRe.FindStringSubmatch(line); len(m) > 1 {
			rawContent := strings.TrimSuffix(m[1], " <nil>")
			// It might be a Go struct representation or JSON depending on how it was logged, 
			// but we can try to pass it as a generic map or just a string. For now, a simple map wrapping a string is safe.
			sessionRaw = append(sessionRaw, map[string]interface{}{"data": rawContent})
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
	
	// Add logs and raw to a wrapper or add them to the Session struct
	// Let's add them to the JSON output by creating a custom response object just for this endpoint.
	resp := struct {
		Session
		Logs []string                 `json:"logs"`
		Raw  []map[string]interface{} `json:"raw"`
	}{
		Session: session,
		Logs:    sessionLogs,
		Raw:     sessionRaw,
	}

	log.Printf("Returning session %s with %d turns", session.ID, len(session.Turns))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
		if strings.Contains(line, "Session Finished.") || 
			strings.Contains(line, "Max Turns Reached.") || 
			strings.Contains(line, "Goal Achieved.") || 
			strings.Contains(line, "Fatal:") {
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow CORS
	},
}

// serveWs handles websocket requests from the peer.
func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:       s.hub,
		sessionID: sessionID,
		conn:      conn,
		send:      make(chan []byte, 256),
	}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()

	// We don't need a readPump because the client only listens in this architecture,
	// but we must read from the connection to process ping/pong/close messages
	// so the connection doesn't silently die.
	go func() {
		defer func() {
			client.hub.unregister <- client
			client.conn.Close()
		}()
		for {
			_, _, err := client.conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}
