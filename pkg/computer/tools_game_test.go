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
	"testing"
)

func TestCustomSkills_Registered(t *testing.T) {
	if !IsToolKnown("hold_key") {
		t.Error("hold_key tool should be registered in tool registry")
	}
	if !IsToolKnown("press_keys") {
		t.Error("press_keys tool should be registered in tool registry")
	}

	decls := GetCustomSkillDeclarations()
	foundHoldKey := false
	foundPressKeys := false

	for _, decl := range decls {
		if decl.Name == "hold_key" {
			foundHoldKey = true
		}
		if decl.Name == "press_keys" {
			foundPressKeys = true
		}
	}

	if !foundHoldKey {
		t.Error("hold_key declaration not found in GetCustomSkillDeclarations()")
	}
	if !foundPressKeys {
		t.Error("press_keys declaration not found in GetCustomSkillDeclarations()")
	}
}
