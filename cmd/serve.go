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

	"github.com/ghchinoy/riptide/pkg/viewer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	GroupID: "viewer",
	Use:     "serve",
	Short:   "Start the Session Viewer web UI",
	Long: `Start the Riptide Session Viewer on a local HTTP port.

The viewer provides a browser-based UI to browse session history, replay
agent reasoning turn-by-turn, and inspect screenshots. It also accepts
live WebSocket events when an agent session is running alongside it
(via 'riptide run --serve').`,
	Example: `  riptide serve
  riptide serve --port 9090
  riptide serve --sessions-dir /var/riptide/sessions`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		port := viper.GetInt("viewer.port")
		if cmd.Flags().Changed("port") {
			port, _ = cmd.Flags().GetInt("port")
		}

		addr := fmt.Sprintf(":%d", port)
		useJSON, _ := cmd.Root().PersistentFlags().GetBool("json")

		if useJSON || os.Getenv("RIPTIDE_NO_TUI") != "" {
			info := map[string]interface{}{
				"status":   "starting",
				"address":  addr,
				"sessions": viper.GetString("sessions.dir"),
			}
			_ = json.NewEncoder(os.Stdout).Encode(info)
		} else {
			fmt.Printf("%s Session Viewer at %s\n",
				stylePass.Render("✓"),
				styleAccent.Render("http://localhost"+addr),
			)
			fmt.Println(muted("  Ctrl+C to stop."))
		}

		return viewer.Start(addr)
	},
}

func init() {
	serveCmd.Flags().Int("port", 0, "Port to listen on (default from config/viewer.port)")
}
