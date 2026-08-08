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

package computer

import (
	"context"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// MatchTargetID searches a list of target.Info for a target matching tabID or tabURLMatch.
// If tabID is non-empty, it converts tabID directly to target.ID.
// If tabURLMatch is non-empty, it returns the TargetID of the first "page" target whose URL contains tabURLMatch.
// If neither matches, or if both are empty, it returns an empty target.ID and nil error (if both empty)
// or an error if a substring was specified but not found.
func MatchTargetID(targets []*target.Info, tabID string, tabURLMatch string) (target.ID, error) {
	if tabID != "" {
		return target.ID(tabID), nil
	}
	if tabURLMatch == "" {
		return "", nil
	}
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, tabURLMatch) {
			return t.TargetID, nil
		}
	}
	return "", fmt.Errorf("no open page target found matching URL substring %q", tabURLMatch)
}

// ResolveTargetID returns the target.ID to attach to based on tabID or tabURLMatch.
// If both are empty, it returns an empty target.ID without error (signaling a new tab should be opened).
// The passed ctx can be a raw allocator context (e.g. from NewRemoteAllocator)
// or an existing chromedp context; a target context will be created as needed.
func ResolveTargetID(ctx context.Context, tabID string, tabURLMatch string) (target.ID, error) {
	if tabID != "" {
		return target.ID(tabID), nil
	}
	if tabURLMatch == "" {
		return "", nil
	}

	// chromedp.Targets requires a full chromedp context created via NewContext,
	// not just a raw allocator context.
	targetCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	targets, err := chromedp.Targets(targetCtx)
	if err != nil {
		return "", fmt.Errorf("failed to query remote targets: %w", err)
	}

	return MatchTargetID(targets, tabID, tabURLMatch)
}
