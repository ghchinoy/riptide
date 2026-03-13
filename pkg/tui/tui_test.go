package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ghchinoy/riptide/pkg/computer"
)

func TestModelUpdate(t *testing.T) {
	// Initialize model
	m := NewModel("test_sessions", "test_id", false, false)

	var retModel tea.Model
	var cmd tea.Cmd

	// Simulate window size message to initialize viewports
	retModel, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = retModel.(Model)
	if !m.ready {
		t.Fatal("Model should be ready after WindowSizeMsg")
	}

	// Test quit key
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("Expected tea.Quit command for 'q' key")
	}

	// Test j key (show JSON)
	retModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = retModel.(Model)
	if !m.showJSON {
		t.Fatal("Expected showJSON to be true")
	}

	// Test esc key to close JSON view
	retModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = retModel.(Model)
	if m.showJSON {
		t.Fatal("Expected showJSON to be false after escape")
	}

	// Test event messages
	// 1. Goal Achieved (no auto-exit)
	retModel, cmd = m.Update(eventMsg{Type: computer.EventStatus, Message: "Goal Achieved."})
	m = retModel.(Model)
	if !m.finished {
		t.Fatal("Expected model to be finished")
	}
	if cmd != nil {
		t.Fatal("Expected no quit command because autoExit is false")
	}

	// 2. Goal Achieved with auto-exit
	m.autoExit = true
	_, cmd = m.Update(eventMsg{Type: computer.EventStatus, Message: "Goal Achieved."})
	if cmd == nil {
		t.Fatal("Expected tea.Quit command since autoExit is true")
	}

	// 3. Event thinking
	retModel, _ = m.Update(eventMsg{Type: computer.EventThinking, Message: "I am thinking"})
	m = retModel.(Model)
	if m.thinking != "🧠 I am thinking" {
		t.Errorf("Expected thinking string to be set, got %s", m.thinking)
	}

	// 4. Event error
	prevLogsLen := len(m.logs)
	retModel, _ = m.Update(eventMsg{Type: computer.EventError, Message: "Something broke"})
	m = retModel.(Model)
	if len(m.logs) <= prevLogsLen {
		t.Fatal("Expected logs to increase")
	}
}