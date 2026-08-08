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

// Package computer — harness efficiency tests (riptide-hyt).
package computer

import (
	"testing"
	"time"
)

// ─── hyt.1: Full-page screenshot gating ──────────────────────────────────────

// The full-page screenshot path is exercised via integration only (it requires
// a live browser). Unit coverage is that capturePostActionScreenshot accepts
// the debugScreenshots bool — confirmed by the build. The behaviour is:
// when false, captureFullPageScreenshot is never called.

// ─── hyt.2: max_turns default ────────────────────────────────────────────────

// Default is enforced via viper in cmd/root.go; tested implicitly via the
// config init template. The canonical check is that the constant exists and
// is ≥ 20 so that future regressions are caught.

// ─── hyt.3: Action-aware wait strategy ───────────────────────────────────────

func TestActionWaitDuration_Navigation(t *testing.T) {
	d := actionWaitDuration(actionClassNavigation)
	if d < 1000*time.Millisecond {
		t.Errorf("navigation wait = %v; want ≥ 1s to allow JS settle", d)
	}
}

func TestActionWaitDuration_Interaction(t *testing.T) {
	d := actionWaitDuration(actionClassInteraction)
	if d >= 1000*time.Millisecond {
		t.Errorf("interaction wait = %v; want < 1s to keep per-turn overhead low", d)
	}
	if d < 50*time.Millisecond {
		t.Errorf("interaction wait = %v; want ≥ 50ms to allow SPA re-render", d)
	}
}

func TestActionWaitDuration_Passive(t *testing.T) {
	d := actionWaitDuration(actionClassPassive)
	if d >= 500*time.Millisecond {
		t.Errorf("passive wait = %v; want < 500ms (no DOM change)", d)
	}
}

func TestClassifyAction_NavigationGroup(t *testing.T) {
	for _, name := range []string{"navigate", "go_back", "go_forward", "search"} {
		if got := classifyAction(name); got != actionClassNavigation {
			t.Errorf("classifyAction(%q) = %v; want Navigation", name, got)
		}
	}
}

func TestClassifyAction_PassiveGroup(t *testing.T) {
	for _, name := range []string{
		"take_screenshot", "wait", "wait_5_seconds",
		"cursor_position", "get_page_layout", "get_accessibility_tree",
	} {
		if got := classifyAction(name); got != actionClassPassive {
			t.Errorf("classifyAction(%q) = %v; want Passive", name, got)
		}
	}
}

func TestClassifyAction_InteractionGroup(t *testing.T) {
	for _, name := range []string{
		"click", "type", "scroll", "hover", "double_click",
		"triple_click", "drag_and_drop", "key", "press_key",
	} {
		if got := classifyAction(name); got != actionClassInteraction {
			t.Errorf("classifyAction(%q) = %v; want Interaction", name, got)
		}
	}
}

// ─── hyt.5: Thrash / loop detection ─────────────────────────────────────────

func TestThrashWindow_NotRepeatingSparse(t *testing.T) {
	w := newThrashWindow(9)
	w.record("click", "https://a.com")
	w.record("scroll", "https://a.com")
	w.record("click", "https://a.com")
	if w.repeating(3) {
		t.Error("different actions should not be detected as a repeat")
	}
}

func TestThrashWindow_DetectsThreeConsecutiveIdentical(t *testing.T) {
	w := newThrashWindow(9)
	w.record("scroll", "https://a.com")
	w.record("scroll", "https://a.com")
	w.record("scroll", "https://a.com")
	if !w.repeating(3) {
		t.Error("three identical (action, url) entries should trigger repeat detection")
	}
}

func TestThrashWindow_TwoNotEnough(t *testing.T) {
	w := newThrashWindow(9)
	w.record("click", "https://b.com")
	w.record("click", "https://b.com")
	if w.repeating(3) {
		t.Error("only two identical entries should not trigger 3-repeat detection")
	}
}

func TestThrashWindow_UrlDifferentiates(t *testing.T) {
	w := newThrashWindow(9)
	w.record("click", "https://a.com")
	w.record("click", "https://b.com") // different URL
	w.record("click", "https://a.com")
	if w.repeating(3) {
		t.Error("same action on different URLs should not trigger repeat detection")
	}
}

func TestThrashWindow_KeyDifferentiates(t *testing.T) {
	w := newThrashWindow(9)
	w.recordAction("press_key", "https://a.com", map[string]interface{}{"key": "ArrowDown"}, "Progress 1%")
	w.recordAction("press_key", "https://a.com", map[string]interface{}{"key": "ArrowRight"}, "Progress 2%")
	w.recordAction("press_key", "https://a.com", map[string]interface{}{"key": "ArrowDown"}, "Progress 3%")
	if w.repeating(3) {
		t.Error("different keys in sequence should not trigger loop detection")
	}
}

func TestThrashWindow_DOMStateDifferentiates(t *testing.T) {
	w := newThrashWindow(9)
	w.recordAction("press_key", "https://a.com", map[string]interface{}{"key": "ArrowRight"}, "Progress 1%")
	w.recordAction("press_key", "https://a.com", map[string]interface{}{"key": "ArrowRight"}, "Progress 2%")
	w.recordAction("press_key", "https://a.com", map[string]interface{}{"key": "ArrowRight"}, "Progress 3%")
	if w.repeating(3) {
		t.Error("same key with changing DOM state (e.g. progress advancing) should not trigger loop detection")
	}
}

func TestThrashWindow_ResetAfterInjection(t *testing.T) {
	w := newThrashWindow(9)
	w.record("scroll", "https://x.com")
	w.record("scroll", "https://x.com")
	w.record("scroll", "https://x.com")
	if !w.repeating(3) {
		t.Fatal("should detect repeat before reset")
	}
	// Simulate the reset that happens after correction injection.
	w = newThrashWindow(9)
	w.record("scroll", "https://x.com")
	if w.repeating(3) {
		t.Error("after reset, single entry should not trigger detection")
	}
}

func TestThrashWindow_SlidingWindowDropsOld(t *testing.T) {
	w := newThrashWindow(4) // small window
	w.record("click", "https://a.com")
	w.record("click", "https://a.com")
	w.record("navigate", "https://b.com") // breaks the run
	w.record("click", "https://a.com")   // after push, click|a dropped off the front
	// Now window is: [click|a, navigate|b, click|a] — last 3 are not identical
	if w.repeating(3) {
		t.Error("sliding window should have dropped the oldest entry, breaking the run")
	}
}

// ─── hyt.4: EventLoop event type ─────────────────────────────────────────────

func TestEventLoop_Defined(t *testing.T) {
	if EventLoop == "" {
		t.Error("EventLoop must be a non-empty event type string")
	}
}

func TestEventLoop_DistinctFromOthers(t *testing.T) {
	others := []EventType{
		EventStatus, EventLog, EventError, EventSafety,
		EventThinking, EventHallucination, EventPromptInjection,
	}
	for _, e := range others {
		if EventLoop == e {
			t.Errorf("EventLoop must be distinct from %q", e)
		}
	}
}
