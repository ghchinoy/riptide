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

// Package cmd implements the Riptide CLI using Cobra and Viper.
// It follows the agent-aware-cli skill: dual DX/AX output, XDG config,
// sane defaults, and --json / NO_COLOR / RIPTIDE_NO_TUI support throughout.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// ConfigFileName is the Viper config file name (without extension).
	ConfigFileName = "config"
	// AppName is used for XDG directory and env prefix.
	AppName = "riptide"
	// DefaultChromeUA is the default browser user-agent.
	DefaultChromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// rootCmd is the base command for the riptide CLI.
var rootCmd = &cobra.Command{
	Use:   "riptide",
	Short: "A Gemini 3.5 Flash agent harness for browser computer use",
	Long: `Riptide is a Go agent harness for Gemini 3.5 Flash with built-in computer use.

It manages the observation–reason–act loop that connects the model to a real
headless Chrome browser via chromedp, handling screenshots, accessibility trees,
tool execution, context pruning, and safety controls.

Get started:
  riptide config init      Create your config at ~/.config/riptide/config.yaml
  riptide run --prompt ""  Run an agent session`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command. Called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, styleFail.Render("Error: ")+err.Error())
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initViper)

	// Command groups for discoverability.
	rootCmd.AddGroup(
		&cobra.Group{ID: "agent", Title: "Agent Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
		&cobra.Group{ID: "viewer", Title: "Session Viewer:"},
	)

	// Persistent flags available to every subcommand.
	pf := rootCmd.PersistentFlags()
	pf.Bool("json", false, "Output machine-readable JSON (AX mode)")
	pf.String("sessions-dir", "sessions", "Directory for session logs and screenshots")

	_ = viper.BindPFlag("sessions.dir", pf.Lookup("sessions-dir"))
	_ = viper.BindPFlag("output.json", pf.Lookup("json"))

	// Register all subcommands.
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(sessionsCmd)
}

// initViper configures Viper for the Riptide CLI.
// Priority order: CLI flag > env var > config file > default.
func initViper() {
	// Env var bindings — these take priority over the config file.
	viper.SetEnvPrefix("")                    // no prefix; use exact env var names
	viper.BindEnv("google.project", "GOOGLE_CLOUD_PROJECT")   //nolint:errcheck
	viper.BindEnv("google.location", "GOOGLE_CLOUD_LOCATION") //nolint:errcheck

	// Defaults.
	// Gemini 3.5 Flash computer use is served from the 'global' location.
	// us-central1 and other regional endpoints return NOT_FOUND for this model.
	viper.SetDefault("google.location", "global")
	viper.SetDefault("session.max_turns", 10)
	viper.SetDefault("session.max_screenshots", 3)
	viper.SetDefault("session.axt", true)
	viper.SetDefault("session.transparent_ua", true)
	viper.SetDefault("session.user_agent", DefaultChromeUA)
	viper.SetDefault("session.gif", false)
	viper.SetDefault("session.show_browser", false)
	viper.SetDefault("session.mode", "default")
	viper.SetDefault("tui.enabled", true)
	viper.SetDefault("tui.quit_on_exit", false)
	viper.SetDefault("tui.high_contrast", false)
	viper.SetDefault("sessions.dir", "sessions")
	viper.SetDefault("viewer.port", 8083)
	viper.SetDefault("model.name", "gemini-3.5-flash")
	viper.SetDefault("model.thinking_budget", 8192)

	// Config file: ~/.config/riptide/config.yaml
	viper.SetConfigName(ConfigFileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(xdgConfigDir())
	viper.AddConfigPath(".")

	// Load config file; missing file is not an error.
	_ = viper.ReadInConfig()

	// Also load legacy .env files for backward compatibility.
	loadLegacyEnv()
}

// xdgConfigDir returns the XDG-compliant config directory for Riptide.
func xdgConfigDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, AppName)
}

// ConfigFilePath returns the full path to the active config file.
func ConfigFilePath() string {
	return filepath.Join(xdgConfigDir(), ConfigFileName+".yaml")
}

// loadLegacyEnv loads variables from .env / XDG .env files without
// overwriting values already set by environment or Viper.
func loadLegacyEnv() {
	for _, path := range []string{
		".env",
		filepath.Join(xdgConfigDir(), ".env"),
	} {
		if data, err := os.ReadFile(path); err == nil {
			for _, line := range splitLines(string(data)) {
				if len(line) == 0 || line[0] == '#' {
					continue
				}
				k, v, ok := cut(line, "=")
				if !ok {
					continue
				}
				k = trimSpace(k)
				v = trimQuotes(trimSpace(v))
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
	}
}

// credentialsHint prints a friendly hint when credentials are missing.
func credentialsHint() {
	fmt.Fprintln(os.Stderr, styleFail.Render("Error:")+" GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION are required.")
	fmt.Fprintln(os.Stderr, styleMuted.Render("  Set them in your environment, or run:")+
		"\n    "+styleCommand.Render("riptide config init")+" — create ~/.config/riptide/config.yaml"+
		"\n    "+styleCommand.Render("riptide config set google.project YOUR_PROJECT_ID"))
}

// Simple string helpers to avoid importing extra packages.
func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '\n' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func cut(s, sep string) (string, string, bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func trimQuotes(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
