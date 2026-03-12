package tui

import (
        "encoding/json"
        "fmt"
        "os"
        "path/filepath"
        "regexp"
        "strings"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/riptide/pkg/computer"
)

type eventMsg computer.Event

type Model struct {
        theme  Theme
        styles Styles

        spinner spinner.Model
	viewport viewport.Model

	jsonViewport viewport.Model

	logViewport viewport.Model

	logs []string

	status string

	thinking string

	action string

	width int

	height int

	ready bool

	        autoExit bool
	
	        sessionsDir string
	
	        sessionID string
	
	        // JSON View
		lastJSON string

	showJSON bool

	showLogs bool

	showHistory bool

	// Safety handling

	finished bool

	safetyPrompt string

	safetyChannel chan bool
}

func NewModel(sessionsDir, sessionID string, autoExit, highContrast bool) Model {
        s := spinner.New()
        s.Spinner = spinner.Dot
        s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

        theme := DefaultTheme()
        if highContrast {
                theme = HighContrastTheme()
        }

        return Model{
                theme:         theme,
                styles:        MakeStyles(theme),
                spinner:       s,
                status:        "Initializing...",
                safetyChannel: make(chan bool),
                autoExit:      autoExit,
                sessionsDir:   sessionsDir,
                sessionID:     sessionID,
        }
}
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showJSON || m.showHistory {
		switch msg.String() {
		case "j", "h", "esc":
			m.showJSON = false
			m.showHistory = false
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.jsonViewport, cmd = m.jsonViewport.Update(msg)
		return m, cmd
	}

	if m.showLogs {
		switch msg.String() {
		case "l", "esc":
			m.showLogs = false
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.logViewport, cmd = m.logViewport.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j":
		m.showJSON = true
		m.jsonViewport.SetContent(m.lastJSON)
		return m, nil
	case "h":
		m.showHistory = true
		// content for jsonViewport will be set by the latest EventRaw labeled "history"
		return m, nil
	case "l":
		m.showLogs = true
		if logData, err := os.ReadFile(filepath.Join(m.sessionsDir, m.sessionID, "session.log")); err == nil {
			m.logViewport.SetContent(string(logData))
			m.logViewport.GotoBottom()
		}
		return m, nil
	case "y", "n":
		if m.safetyPrompt != "" {
			m.safetyChannel <- (msg.String() == "y")
			m.safetyPrompt = ""
			m.status = "Safety decision sent."
		}
	}
	return m, nil
}

func (m Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	headerHeight := 8
	footerHeight := 4
	m.width = msg.Width
	m.height = msg.Height

	if !m.ready {
		m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
		m.jsonViewport = viewport.New(msg.Width-4, msg.Height-8)
		m.logViewport = viewport.New(msg.Width-4, msg.Height-8)
		m.ready = true
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight
		m.jsonViewport.Width = msg.Width - 4
		m.jsonViewport.Height = msg.Height - 8
		m.logViewport.Width = msg.Width - 4
		m.logViewport.Height = msg.Height - 8
	}
	return m, nil
}

func (m Model) handleEventMsg(msg eventMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case computer.EventStatus:
		icon := "ℹ️ "
		switch msg.Message {
		case "Goal Achieved.", "Session Finished.":
			icon = "✅ "
			m.finished = true
			if m.autoExit {
				return m, tea.Quit
			}
		case "Max Turns Reached.":
			icon = "🛑 "
			m.finished = true
			if m.autoExit {
				return m, tea.Quit
			}
		}
		m.status = icon + msg.Message

		if strings.HasPrefix(msg.Message, "Turn") {
			m.thinking = ""
			m.action = ""
		}
	case computer.EventThinking:
		m.thinking = "🧠 " + msg.Message
		wrapped := lipgloss.NewStyle().Width(m.viewport.Width - 2).Render("🧠 " + msg.Message)
		m.logs = append(m.logs, m.styles.Thinking.Render(wrapped))
	case computer.EventAction:
		m.action = "🛠️ " + msg.Message
		wrapped := lipgloss.NewStyle().Width(m.viewport.Width - 2).Render(fmt.Sprintf("🛠️ Action: %s", msg.Message))
		m.logs = append(m.logs, m.styles.Action.Render(wrapped))
	case computer.EventLog:
		wrapped := lipgloss.NewStyle().Width(m.viewport.Width - 2).Render("📄 " + msg.Message)
		m.logs = append(m.logs, m.styles.Info.Render(wrapped))

	case computer.EventRaw:
		if b, err := json.MarshalIndent(msg.Data, "", "  "); err == nil {
			s := string(b)
			// Truncate base64 strings (usually data:image/png;base64,...)
			re := regexp.MustCompile(`"data":\s*"[^"]{100,}"`)
			s = re.ReplaceAllString(s, `"data": "<base64 truncated>"`)

			m.lastJSON = s
			m.jsonViewport.SetContent(m.lastJSON)
		}
	case computer.EventError:
		m.logs = append(m.logs, m.styles.Error.Render(fmt.Sprintf("❌ Error: %s", msg.Message)))
	case computer.EventSafety:
		m.safetyPrompt = "🚨 " + msg.Message
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
	return m, nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)
	case eventMsg:
		return m.handleEventMsg(msg)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}
		
		func (m Model) View() string {
		        if !m.ready {
		                return "\n  Initializing..."
		        }
		
		        header := m.styles.Title.Render(" Gemini Computer Use Agent ")
		
		        icon := m.spinner.View()
		        if m.finished {
		                icon = "✓"
		        }
		        status := fmt.Sprintf("\n  %s %s", icon, m.styles.Status.Render(m.status))
		
		        thinking := ""
		        if m.thinking != "" {
		                // Truncate to one line for the header
		                txt := m.thinking
		                if idx := strings.Index(txt, "\n"); idx != -1 {
		                        txt = txt[:idx] + "..."
		                }
		                thinking = fmt.Sprintf("\n  %s", m.styles.Thinking.MaxWidth(m.width-4).Render(txt))
		        }
		
		        action := ""
		        if m.action != "" {
		                action = fmt.Sprintf("\n  %s", m.styles.Action.MaxWidth(m.width-4).Render(m.action))
		        }
		
		        safety := ""
		        if m.safetyPrompt != "" {
		                safety = m.styles.Safety.Render(fmt.Sprintf("SAFETY ALERT: %s\nProceed? (y/n)", m.safetyPrompt))
		                safety = "\n\n" + safety
		        }
			        help := m.styles.Info.Render(fmt.Sprintf("\n  q: quit | j: json | l: logs | h: hist | s: %s", m.sessionID))
				mainView := fmt.Sprintf("%s%s%s%s\n\n%s%s%s",
		header, status, thinking, action,
		m.viewport.View(),
		safety,
		help)

	if m.showHistory {
		histHeader := m.styles.Title.Render(" Conversation History (Request) ")
		                histFooter := m.styles.Info.Render("\n  h/esc: back | arrows: scroll")
				wrapped := lipgloss.NewStyle().Width(m.width - 6).Render(m.lastJSON)
		m.jsonViewport.SetContent(wrapped)

		                content := fmt.Sprintf("%s\n\n%s%s", histHeader, m.jsonViewport.View(), histFooter)

		                return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,

		                        lipgloss.NewStyle().Width(m.width-2).Height(m.height-4).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(m.theme.BorderView)).Padding(1).Render(content))

		        }

		

		        if m.showJSON {

		                jsonHeader := m.styles.Title.Render(" Raw Model JSON (Response) ")

		                jsonFooter := m.styles.Info.Render("\n  j/esc: back | arrows: scroll")

		

		                wrapped := lipgloss.NewStyle().Width(m.width - 6).Render(m.lastJSON)

		                m.jsonViewport.SetContent(wrapped)

		

		                content := fmt.Sprintf("%s\n\n%s%s", jsonHeader, m.jsonViewport.View(), jsonFooter)

		                return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,

		                        lipgloss.NewStyle().Width(m.width-2).Height(m.height-4).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(m.theme.BorderJSON)).Padding(1).Render(content))

		        }

		

		        if m.showLogs {

		                logHeader := m.styles.Title.Render(" Session Logs ")

		                logFooter := m.styles.Info.Render("\n  l/esc: back | arrows: scroll")

		

		                // Load log file

		                if logData, err := os.ReadFile(filepath.Join(m.sessionsDir, m.sessionID, "session.log")); err == nil {

		                        wrapped := lipgloss.NewStyle().Width(m.width - 6).Render(string(logData))

		                        m.logViewport.SetContent(wrapped)

		                }

		

		                content := fmt.Sprintf("%s\n\n%s%s", logHeader, m.logViewport.View(), logFooter)

		                return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,

		                        lipgloss.NewStyle().Width(m.width-2).Height(m.height-4).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(m.theme.BorderLog)).Padding(1).Render(content))

		        }

		

	return mainView
}

// GetObserver returns an observer function that sends events to the tea program.
func (m Model) GetObserver(p *tea.Program) computer.Observer {
	return func(e computer.Event) {
		p.Send(eventMsg(e))
	}
}

// GetSafetyHandler returns a safety handler that uses the UI to get confirmation.
func (m Model) GetSafetyHandler(p *tea.Program) computer.SafetyHandler {
	return func(explanation string) bool {
		p.Send(eventMsg{
			Type:    computer.EventSafety,
			Message: explanation,
		})
		return <-m.safetyChannel
	}
}
