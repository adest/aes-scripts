package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {

	rootCmd = &cobra.Command{
		Use:   "go-tools",
		Short: "A collection of Go tools",
	}

	rootCmd.AddCommand(newBuildCommand())
	rootCmd.AddCommand(newCleanCommand())
	rootCmd.AddCommand(newInstallCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
	}
}