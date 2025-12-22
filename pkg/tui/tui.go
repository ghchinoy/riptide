package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ghchinoy/website-assistant/pkg/computer"
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
	spinner  spinner.Model
	viewport viewport.Model
	logs     []string
	status   string
	thinking string
	action   string
	width    int
	height   int
	ready    bool
	
	// Safety handling
	safetyPrompt  string
	safetyChannel chan bool
}

func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		spinner:       s,
		status:        "Initializing...",
		safetyChannel: make(chan bool),
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight
		}

	case eventMsg:
		switch msg.Type {
		case computer.EventStatus:
			m.status = msg.Message
		case computer.EventThinking:
			m.thinking = msg.Message
		case computer.EventAction:
			m.action = msg.Message
			m.logs = append(m.logs, actionStyle.Render(fmt.Sprintf("Action: %s", msg.Message)))
		case computer.EventLog:
			m.logs = append(m.logs, infoStyle.Render(msg.Message))
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
	status := fmt.Sprintf("\n  %s %s", m.spinner.View(), statusStyle.Render(m.status))
	
	thinking := ""
	if m.thinking != "" {
		thinking = fmt.Sprintf("\n  %s", thinkingStyle.Render(m.thinking))
	}
	
action := ""
	if m.action != "" {
		action = fmt.Sprintf("\n  %s", actionStyle.Render(m.action))
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

	help := infoStyle.Render("\n  q: quit")

	return fmt.Sprintf("%s%s%s%s\n\n%s%s%s", 
		header, status, thinking, action, 
		m.viewport.View(), 
		safety,
		help)
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