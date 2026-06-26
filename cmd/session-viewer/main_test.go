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

// Package main — tests for the session-viewer binary entry point.
// The session parsing logic previously inlined here has been moved to
// pkg/results (parser_test.go) and pkg/viewer (server.go). These tests verify
// the binary compiles and the package structure is sound.
package main

import "testing"

// TestMain_PackageCompiles is a sentinel that fails at build time if the
// package cannot be compiled, without requiring any runtime state.
func TestMain_PackageCompiles(t *testing.T) {
	// The session-viewer binary delegates all logic to pkg/viewer.
	// Parsing correctness is tested in pkg/results.
	// HTTP endpoint correctness is tested implicitly by pkg/viewer's test coverage.
	t.Log("session-viewer package compiles successfully")
}
