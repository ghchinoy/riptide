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

package utils

import (

	"bufio"

	"os"

	"path/filepath"

	"strings"

)





// LoadConfig attempts to load environment variables from multiple locations:

// 1. A local .env file in the current directory.

// 2. $XDG_CONFIG_HOME/riptide/.env

// 3. ~/.config/riptide/.env (if XDG_CONFIG_HOME is not set)

func LoadConfig() {

	// 1. Local .env

	_ = LoadEnv(".env")



	// 2. XDG Config

	configDir := os.Getenv("XDG_CONFIG_HOME")

	if configDir == "" {

		home, err := os.UserHomeDir()

		if err == nil {

			configDir = filepath.Join(home, ".config")

		}

	}



	if configDir != "" {

		xdgEnv := filepath.Join(configDir, "riptide", ".env")

		_ = LoadEnv(xdgEnv)

	}

}



// LoadEnv loads environment variables from a .env file if it exists.


// It does not overwrite existing environment variables.
func LoadEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}

	return scanner.Err()
}
