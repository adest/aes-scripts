package main

import (
	"os"
	"path/filepath"
)

func goToolsBaseDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "go-tools")
}

func goToolsBinDir() string {
	return filepath.Join(goToolsBaseDir(), "bin")
}

func completionsBaseDir() string {
	return filepath.Join(goToolsBaseDir(), "completions")
}

func completionsDirFor(shell string) string {
	return filepath.Join(completionsBaseDir(), shell)
}
