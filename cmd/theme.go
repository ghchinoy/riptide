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

package cmd

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// noColor reports whether colour output should be suppressed.
// Respects the NO_COLOR convention (https://no-color.org/) and the
// RIPTIDE_NO_TUI env var used for fully headless / AX mode.
func noColor() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("RIPTIDE_NO_TUI") != ""
}

// Semantic colour tokens following the agent-aware-cli skill.
// All styles degrade to plain text when noColor() is true.
var (
	// Accent (blue) — navigation landmarks, headers, group titles.
	styleAccent = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
	}()

	// Command (grey) — command names and flags.
	styleCommand = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	}()

	// Pass (green) — success states and completed values.
	stylePass = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	}()

	// Warn (yellow) — pending states and warnings.
	styleWarn = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	}()

	// Fail (red) — errors and rejected states.
	styleFail = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	}()

	// Muted (dark grey) — de-emphasis for metadata, defaults.
	styleMuted = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}()

	// ID (teal) — unique identifiers and session IDs.
	styleID = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	}()

	// Section header separator line.
	styleSeparator = func() lipgloss.Style {
		if noColor() {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	}()
)

// header renders a bold accented section title.
func header(s string) string { return styleAccent.Render(s) }

// muted renders de-emphasised metadata text.
func muted(s string) string { return styleMuted.Render(s) }

// separator returns a horizontal rule string.
func separator(width int) string {
	line := ""
	for i := 0; i < width; i++ {
		line += "─"
	}
	return styleSeparator.Render(line)
}

// provenanceLabel renders the source of a config value (env / file / default).
func provenanceLabel(src string) string {
	switch src {
	case "env":
		return stylePass.Render("env")
	case "file":
		return styleAccent.Render("file")
	case "flag":
		return styleWarn.Render("flag")
	default:
		return styleMuted.Render("default")
	}
}
