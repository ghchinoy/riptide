package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	// Create a temp directory for tests
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	
	envContent := `
# This is a comment
KEY1=value1
KEY2="value2"
KEY3='value3'
  KEY4  =  spaced_value  
EMPTY=
=invalid
`
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write test env file: %v", err)
	}

	// Make sure keys are unset
	os.Unsetenv("KEY1")
	os.Unsetenv("KEY2")
	os.Unsetenv("KEY3")
	os.Unsetenv("KEY4")
	os.Unsetenv("PRE_EXISTING")

	// Set a pre-existing env var
	os.Setenv("PRE_EXISTING", "original")

	// Test non-existent file
	err := LoadEnv(filepath.Join(tempDir, "does-not-exist.env"))
	if err != nil {
		t.Errorf("LoadEnv should return nil for non-existent file, got %v", err)
	}

	// Write PRE_EXISTING to the env file as well to test it's not overwritten
	appendContent := "\nPRE_EXISTING=new_value\n"
	f, _ := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(appendContent)
	f.Close()

	// Load the env file
	err = LoadEnv(envPath)
	if err != nil {
		t.Errorf("LoadEnv returned error: %v", err)
	}

	tests := map[string]string{
		"KEY1":         "value1",
		"KEY2":         "value2",
		"KEY3":         "value3",
		"KEY4":         "spaced_value",
		"PRE_EXISTING": "original",
	}

	for k, want := range tests {
		got := os.Getenv(k)
		if got != want {
			t.Errorf("For %s, expected %q, got %q", k, want, got)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	// Setup mock directories
	tempDir := t.TempDir()
	
	// Create XDG_CONFIG_HOME
	xdgDir := filepath.Join(tempDir, "config")
	os.MkdirAll(filepath.Join(xdgDir, "riptide"), 0755)
	xdgEnv := filepath.Join(xdgDir, "riptide", ".env")
	os.WriteFile(xdgEnv, []byte("XDG_KEY=xdg_val\n"), 0644)

	// Create local .env
	cwd, _ := os.Getwd()
	localEnv := filepath.Join(cwd, ".env")
	// Make sure we clean up if we write to it
	defer os.Remove(localEnv)
	os.WriteFile(localEnv, []byte("LOCAL_KEY=local_val\n"), 0644)

	os.Unsetenv("LOCAL_KEY")
	os.Unsetenv("XDG_KEY")
	
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)
	os.Setenv("XDG_CONFIG_HOME", xdgDir)

	LoadConfig()

	if os.Getenv("LOCAL_KEY") != "local_val" {
		t.Errorf("LOCAL_KEY not loaded, got %q", os.Getenv("LOCAL_KEY"))
	}
	if os.Getenv("XDG_KEY") != "xdg_val" {
		t.Errorf("XDG_KEY not loaded, got %q", os.Getenv("XDG_KEY"))
	}
}