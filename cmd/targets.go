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
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var targetsCmd = &cobra.Command{
	GroupID: "agent",
	Use:     "targets",
	Short:   "List open windows/tabs from an existing Chrome instance",
	Long: `Targets queries a running Chrome instance (started with --remote-debugging-port)
and displays a list of open targets (tabs, windows, service workers).

Pass the resulting Target ID to 'riptide run --attach <url> --tab-id <id>' or use a
URL substring with 'riptide run --attach <url> --tab-url-match <substring>'.`,
	Example: `  # List open tabs from Chrome running at default debugging port
  riptide targets --cdp-url http://localhost:9222

  # Show all target types including background pages / service workers
  riptide targets --cdp-url http://localhost:9222 --all`,
	RunE: runTargets,
}

func init() {
	f := targetsCmd.Flags()
	f.String("cdp-url", "", "CDP HTTP or WebSocket URL of the Chrome instance (default: config/browser.attach_url or http://127.0.0.1:9222)")
	f.Bool("all", false, "Show all target types (default shows only 'page' targets / tabs)")

	rootCmd.AddCommand(targetsCmd)
}

func runTargets(cmd *cobra.Command, _ []string) error {
	flags := cmd.Flags()
	cdpURL, _ := flags.GetString("cdp-url")
	if cdpURL == "" {
		cdpURL = viper.GetString("browser.attach_url")
	}
	if cdpURL == "" {
		cdpURL = "http://127.0.0.1:9222"
	}
	showAll, _ := flags.GetBool("all")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, cdpURL)
	defer allocCancel()

	targetCtx, targetCancel := chromedp.NewContext(allocCtx)
	defer targetCancel()

	targets, err := chromedp.Targets(targetCtx)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp") || strings.Contains(msg, "no such host") || strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "EOF") {
			return fmt.Errorf("could not connect to Chrome DevTools Protocol at %s (%v)\nEnsure Chrome was launched with '--remote-debugging-port' (see README for launch commands)", cdpURL, err)
		}
		return fmt.Errorf("failed to query CDP targets at %s: %w", cdpURL, err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TARGET ID\tTYPE\tATTACHED\tTITLE\tURL")

	count := 0
	for _, t := range targets {
		if !showAll && t.Type != "page" {
			continue
		}
		count++
		attached := "no"
		if t.Attached {
			attached = "yes"
		}
		title := t.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		urlStr := t.URL
		if len(urlStr) > 60 {
			urlStr = urlStr[:57] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.TargetID, t.Type, attached, title, urlStr)
	}
	_ = w.Flush()

	if count == 0 {
		if !showAll {
			fmt.Println("\nNo open page targets found. Use --all to show non-page targets.")
		} else {
			fmt.Println("\nNo targets found.")
		}
	}
	return nil
}
