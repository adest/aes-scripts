package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

// completionExcludeList lists tools that are known not to support shell completions
// (e.g. interactive TUI tools). Add entries here to skip them during completion
// generation without triggering the timeout-based detection.
var completionExcludeList = []string{
	"die",
	"kk",
}

func init() {

	rootCmd = &cobra.Command{
		Use:   "go-tools",
		Short: "A collection of Go tools",
	}

	rootCmd.AddCommand(newBuildCommand())
	rootCmd.AddCommand(newCleanCommand())
	rootCmd.AddCommand(newInstallCommand())
	rootCmd.AddCommand(newCompletionsCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
	}
}