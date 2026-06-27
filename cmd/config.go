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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	GroupID: "config",
	Use:     "config",
	Short:   "Manage Riptide configuration",
	Long: `Manage the Riptide configuration file at ~/.config/riptide/config.yaml.

Configuration priority (highest to lowest):
  1. Command-line flags
  2. Environment variables  (GOOGLE_CLOUD_PROJECT, GOOGLE_CLOUD_LOCATION, …)
  3. Config file            (~/.config/riptide/config.yaml)
  4. Built-in defaults`,
}

// configInitCmd creates a default config file.
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config file at the XDG config path",
	Long:  `Creates ~/.config/riptide/config.yaml with all settings and their defaults.`,
	Example: `  riptide config init
  riptide config init --force   # Overwrite an existing config`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		force, _ := cmd.Flags().GetBool("force")
		path := ConfigFilePath()

		if _, err := os.Stat(path); err == nil && !force {
			fmt.Fprintf(os.Stderr, "%s Config already exists at %s\n",
				styleWarn.Render("Warning:"), styleCommand.Render(path))
			fmt.Fprintln(os.Stderr, muted("  Use --force to overwrite, or 'riptide config set KEY VALUE' to update a single key."))
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		content := fmt.Sprintf(`# Riptide configuration
# https://github.com/ghchinoy/riptide
#
# Priority: env vars > this file > built-in defaults.
# Run 'riptide config show' to see the effective merged configuration.

# Google Cloud credentials
# You can also set GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION env vars.
google:
  project: ""
  location: "global"  # 3.5 Flash computer use is only served from 'global'

# Model settings
model:
  name: "%s"
  thinking_budget: %d

# Agent session defaults
session:
  max_turns: %d      # Raised to 20 to match Python reference; complex SPAs need 15-20 turns
  max_screenshots: %d
  axt: %v
  transparent_ua: %v
  user_agent: "%s"
  gif: false
  show_browser: false
  mode: "default"

# TUI settings
tui:
  enabled: %v
  quit_on_exit: false
  high_contrast: false

# Sessions storage
sessions:
  dir: "sessions"

# Session Viewer
viewer:
  port: %d
`,
			viper.GetString("model.name"),
			viper.GetInt("model.thinking_budget"),
			viper.GetInt("session.max_turns"),
			viper.GetInt("session.max_screenshots"),
			viper.GetBool("session.axt"),
			viper.GetBool("session.transparent_ua"),
			viper.GetString("session.user_agent"),
			viper.GetBool("tui.enabled"),
			viper.GetInt("viewer.port"),
		)

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Printf("%s Config created at %s\n", stylePass.Render("✓"), styleCommand.Render(path))
		fmt.Println(muted("  Edit it directly, or use 'riptide config set KEY VALUE'."))
		return nil
	},
}

// configShowCmd displays the effective merged configuration.
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the effective configuration with source provenance",
	Long:  `Displays all configuration keys with their current values and where each value came from (env / file / default).`,
	Example: `  riptide config show
  riptide config show --json`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")
		if os.Getenv("RIPTIDE_NO_TUI") != "" {
			useJSON = true
		}

		type entry struct {
			Key        string `json:"key"`
			Value      string `json:"value"`
			Provenance string `json:"provenance"`
		}

		keys := []string{
			"google.project", "google.location",
			"model.name", "model.thinking_budget",
			"session.max_turns", "session.max_screenshots",
			"session.axt", "session.transparent_ua",
			"session.gif", "session.show_browser", "session.mode",
			"tui.enabled", "tui.quit_on_exit", "tui.high_contrast",
			"sessions.dir",
			"viewer.port",
		}

		// Determine provenance for each key.
		cfgFile := viper.ConfigFileUsed()
		viperCfg := viper.AllSettings()
		flatCfg := flattenMap(viperCfg, "")

		entries := make([]entry, 0, len(keys))
		for _, k := range keys {
			val := fmt.Sprintf("%v", viper.Get(k))
			prov := "default"
			// Check env vars for credential keys.
			if k == "google.project" && os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
				prov = "env"
			} else if k == "google.location" && os.Getenv("GOOGLE_CLOUD_LOCATION") != "" {
				prov = "env"
			} else if cfgFile != "" {
				if _, found := flatCfg[k]; found {
					prov = "file"
				}
			}
			entries = append(entries, entry{Key: k, Value: val, Provenance: prov})
		}

		if useJSON {
			type output struct {
				ConfigFile string  `json:"config_file"`
				Settings   []entry `json:"settings"`
			}
			return json.NewEncoder(os.Stdout).Encode(output{
				ConfigFile: cfgFile,
				Settings:   entries,
			})
		}

		// Human-readable output.
		fmt.Println()
		fmt.Println(header("Riptide Configuration"))
		fmt.Println(separator(50))

		sections := []struct {
			title string
			keys  []string
		}{
			{"Credentials", []string{"google.project", "google.location"}},
			{"Model", []string{"model.name", "model.thinking_budget"}},
			{"Session Defaults", []string{
				"session.max_turns", "session.max_screenshots",
				"session.axt", "session.transparent_ua", "session.gif",
				"session.show_browser", "session.mode",
			}},
			{"TUI", []string{"tui.enabled", "tui.quit_on_exit", "tui.high_contrast"}},
			{"Storage", []string{"sessions.dir"}},
			{"Viewer", []string{"viewer.port"}},
		}

		entryMap := make(map[string]entry, len(entries))
		for _, e := range entries {
			entryMap[e.Key] = e
		}

		for _, sec := range sections {
			fmt.Println()
			fmt.Println(styleAccent.Render(sec.title))
			for _, k := range sec.keys {
				e := entryMap[k]
				// Pad key to 28 chars before applying colour (alignment-safe).
				paddedKey := fmt.Sprintf("  %-28s", k)
				paddedVal := fmt.Sprintf("%-20s", e.Value)
				fmt.Printf("%s %s %s\n",
					styleCommand.Render(paddedKey),
					paddedVal,
					provenanceLabel(e.Provenance),
				)
			}
		}

		fmt.Println()
		if cfgFile != "" {
			fmt.Printf("%s %s\n", muted("Config file:"), styleCommand.Render(cfgFile))
		} else {
			fmt.Printf("%s %s\n",
				muted("Config file:"),
				styleWarn.Render("not found — run 'riptide config init' to create one"),
			)
		}
		fmt.Println()
		return nil
	},
}

// configSetCmd updates a single key in the config file.
var configSetCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set a configuration key in the config file",
	Long:  `Set a key in ~/.config/riptide/config.yaml. Creates the file if it does not exist.`,
	Example: `  riptide config set google.project my-gcp-project
  riptide config set session.max_turns 20
  riptide config set tui.high_contrast true`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		path := ConfigFilePath()

		// Ensure the config file exists.
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(""), 0644); err != nil {
				return err
			}
		}

		// Set via Viper and write back.
		viper.SetConfigFile(path)
		if err := viper.ReadInConfig(); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not read config: %w", err)
		}
		viper.Set(key, value)
		if err := viper.WriteConfigAs(path); err != nil {
			return fmt.Errorf("could not write config: %w", err)
		}

		fmt.Printf("%s %s = %s\n",
			stylePass.Render("✓"),
			styleCommand.Render(key),
			value,
		)
		return nil
	},
}

// configPathCmd prints the config file path.
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the active config file path",
	RunE: func(_ *cobra.Command, _ []string) error {
		path := ConfigFilePath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("%s %s\n", styleWarn.Render("(not found)"), path)
		} else {
			fmt.Println(path)
		}
		return nil
	},
}

func init() {
	configInitCmd.Flags().Bool("force", false, "Overwrite an existing config file")
	configCmd.AddCommand(configInitCmd, configShowCmd, configSetCmd, configPathCmd)
}

// flattenMap flattens a nested map to dot-separated keys.
func flattenMap(m map[string]interface{}, prefix string) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		if sub, ok := v.(map[string]interface{}); ok {
			for kk, vv := range flattenMap(sub, full) {
				out[kk] = vv
			}
		} else {
			out[full] = v
		}
	}
	return out
}

// sortedKeys returns a sorted key slice from a map.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// unused but kept for potential future use.
var _ = strings.Join
var _ = sortedKeys
