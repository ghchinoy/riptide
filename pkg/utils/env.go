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

	LoadEnv(".env")



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

		LoadEnv(xdgEnv)

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
	defer file.Close()

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
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}
