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

package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the TUI.
type Theme struct {
	Primary          string
	Secondary        string
	Thinking         string
	Action           string
	Info             string
	Error            string
	Success          string
	Warning          string
	Background       string
	Foreground       string
	BorderView       string
	BorderJSON       string
	BorderLog        string
}

// Styles contains the lipgloss styles derived from a Theme.
type Styles struct {
	Title    lipgloss.Style
	Status   lipgloss.Style
	Thinking lipgloss.Style
	Action   lipgloss.Style
	Info     lipgloss.Style
	Error    lipgloss.Style
	Safety   lipgloss.Style
	Help     lipgloss.Style
}

// DefaultTheme returns the standard Riptide theme.
func DefaultTheme() Theme {
	return Theme{
		Primary:    "#7D56F4", // Purple
		Secondary:  "#FAFAFA", // White
		Thinking:   "#888888", // Gray
		Action:     "#FF5F87", // Pink
		Info:       "#3C3C3C", // Dark Gray
		Error:      "#FF0000", // Red
		Success:    "#04B575", // Green
		Warning:    "#FFB000", // Orange/Amber
		Foreground: "#FAFAFA",
		BorderView: "99",
		BorderJSON: "62",
		BorderLog:  "212",
	}
}

// HighContrastTheme returns a theme with improved accessibility.
func HighContrastTheme() Theme {
	return Theme{
		Primary:    "#FFFFFF",
		Secondary:  "#000000",
		Thinking:   "#CCCCCC",
		Action:     "#FFFF00", // Yellow
		Info:       "#FFFFFF",
		Error:      "#FF0000",
		Success:    "#00FF00",
		Warning:    "#FFA500",
		Foreground: "#FFFFFF",
		BorderView: "15",
		BorderJSON: "15",
		BorderLog:  "15",
	}
}

// MakeStyles generates lipgloss styles from a given Theme.
func MakeStyles(t Theme) Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Foreground)).
			Background(lipgloss.Color(t.Primary)).
			Padding(0, 1),

		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Success)).
			Bold(true),

		Thinking: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Thinking)).
			Italic(true),

		Action: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Action)).
			Bold(true),

		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Info)),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Error)),

		Safety: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Error)).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(1).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Info)),
	}
}
