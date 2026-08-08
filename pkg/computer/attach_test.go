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
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func TestMatchTargetID_DirectTabID(t *testing.T) {
	got, err := MatchTargetID(nil, "ABC-123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ABC-123" {
		t.Errorf("got %q, want ABC-123", got)
	}
}

func TestMatchTargetID_BothEmpty(t *testing.T) {
	got, err := MatchTargetID(nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestMatchTargetID_URLMatchSuccess(t *testing.T) {
	targets := []*target.Info{
		{TargetID: "t1", Type: "service_worker", URL: "https://example.com/sw.js"},
		{TargetID: "t2", Type: "page", URL: "https://google.com/search"},
		{TargetID: "t3", Type: "page", URL: "https://example.com/dashboard"},
	}

	got, err := MatchTargetID(targets, "", "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "t3" {
		t.Errorf("got %q, want t3", got)
	}
}

func TestMatchTargetID_URLMatchNotFound(t *testing.T) {
	targets := []*target.Info{
		{TargetID: "t1", Type: "page", URL: "https://google.com"},
	}

	_, err := MatchTargetID(targets, "", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveTargetID_NotInvalidContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use an unallocated local port where Chrome is not running
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, "http://127.0.0.1:59999")
	defer allocCancel()

	_, err := ResolveTargetID(allocCtx, "", "example.com")
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if strings.Contains(err.Error(), "invalid context") {
		t.Fatalf("ResolveTargetID failed with 'invalid context'; wanted real dial/connection error, got: %v", err)
	}
}
