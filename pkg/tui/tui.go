package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/riptide/pkg/computer"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	actionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F87")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3C3C3C"))
)

type eventMsg computer.Event

type Model struct {
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

func NewModel(sessionID string, autoExit bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		spinner:       s,
		status:        "Initializing...",
		safetyChannel: make(chan bool),
		autoExit:      autoExit,
		sessionID:     sessionID,
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
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
			if logData, err := os.ReadFile(fmt.Sprintf("logs/session_%s.log", m.sessionID)); err == nil {
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

	case tea.WindowSizeMsg:
		headerHeight := 8
		footerHeight := 4
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.viewport.HighPerformanceRendering = false

			m.jsonViewport = viewport.New(msg.Width-4, msg.Height-8)
			m.jsonViewport.HighPerformanceRendering = false

			m.logViewport = viewport.New(msg.Width-4, msg.Height-8)
			m.logViewport.HighPerformanceRendering = false

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight
			m.jsonViewport.Width = msg.Width - 4
			m.jsonViewport.Height = msg.Height - 8
			m.logViewport.Width = msg.Width - 4
			m.logViewport.Height = msg.Height - 8
		}

	case eventMsg:
		switch msg.Type {
		case computer.EventStatus:
			m.status = msg.Message
			if msg.Message == "Session Finished." {
				m.finished = true
				if m.autoExit {
					return m, tea.Quit
				}
			}
			if strings.HasPrefix(msg.Message, "Turn") {
				m.thinking = ""
				m.action = ""
			}
		case computer.EventThinking:
			m.thinking = msg.Message
			wrapped := lipgloss.NewStyle().Width(m.viewport.Width - 2).Render(msg.Message)
			m.logs = append(m.logs, thinkingStyle.Render(wrapped))
		case computer.EventAction:
			m.action = msg.Message
			wrapped := lipgloss.NewStyle().Width(m.viewport.Width - 2).Render(fmt.Sprintf("Action: %s", msg.Message))
			m.logs = append(m.logs, actionStyle.Render(wrapped))
		case computer.EventLog:
			wrapped := lipgloss.NewStyle().Width(m.viewport.Width - 2).Render(msg.Message)
			m.logs = append(m.logs, infoStyle.Render(wrapped))
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
			m.logs = append(m.logs, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(fmt.Sprintf("Error: %s", msg.Message)))
		case computer.EventSafety:
			m.safetyPrompt = msg.Message
		}
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := titleStyle.Render(" Gemini Computer Use Agent ")

	icon := m.spinner.View()
	if m.finished {
		icon = "✓"
	}
	status := fmt.Sprintf("\n  %s %s", icon, statusStyle.Render(m.status))

	thinking := ""
	if m.thinking != "" {
		// Truncate to one line for the header
		txt := m.thinking
		if idx := strings.Index(txt, "\n"); idx != -1 {
			txt = txt[:idx] + "..."
		}
		thinking = fmt.Sprintf("\n  %s", thinkingStyle.MaxWidth(m.width-4).Render(txt))
	}

	action := ""
	if m.action != "" {
		action = fmt.Sprintf("\n  %s", actionStyle.MaxWidth(m.width-4).Render(m.action))
	}

	safety := ""
	if m.safetyPrompt != "" {
		safety = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(1).
			Bold(true).
			Render(fmt.Sprintf("SAFETY ALERT: %s\nProceed? (y/n)", m.safetyPrompt))
		safety = "\n\n" + safety
	}

	help := infoStyle.Render(fmt.Sprintf("\n  q: quit | j: json | l: logs | h: hist | s: %s", m.sessionID))

	mainView := fmt.Sprintf("%s%s%s%s\n\n%s%s%s",
		header, status, thinking, action,
		m.viewport.View(),
		safety,
		help)

	if m.showHistory {
		histHeader := titleStyle.Render(" Conversation History (Request) ")
		histFooter := infoStyle.Render("\n  h/esc: back | arrows: scroll")

		wrapped := lipgloss.NewStyle().Width(m.width - 6).Render(m.lastJSON)
		m.jsonViewport.SetContent(wrapped)

		content := fmt.Sprintf("%s\n\n%s%s", histHeader, m.jsonViewport.View(), histFooter)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Width(m.width-2).Height(m.height-4).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("99")).Padding(1).Render(content))
	}

	if m.showJSON {
		jsonHeader := titleStyle.Render(" Raw Model JSON (Response) ")
		jsonFooter := infoStyle.Render("\n  j/esc: back | arrows: scroll")

		wrapped := lipgloss.NewStyle().Width(m.width - 6).Render(m.lastJSON)
		m.jsonViewport.SetContent(wrapped)

		content := fmt.Sprintf("%s\n\n%s%s", jsonHeader, m.jsonViewport.View(), jsonFooter)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Width(m.width-2).Height(m.height-4).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1).Render(content))
	}

	if m.showLogs {
		logHeader := titleStyle.Render(" Session Logs ")
		logFooter := infoStyle.Render("\n  l/esc: back | arrows: scroll")

		// Load log file
		if logData, err := os.ReadFile(fmt.Sprintf("logs/session_%s.log", m.sessionID)); err == nil {
			wrapped := lipgloss.NewStyle().Width(m.width - 6).Render(string(logData))
			m.logViewport.SetContent(wrapped)
		}

		content := fmt.Sprintf("%s\n\n%s%s", logHeader, m.logViewport.View(), logFooter)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Width(m.width-2).Height(m.height-4).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("212")).Padding(1).Render(content))
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
