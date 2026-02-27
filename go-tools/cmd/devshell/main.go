package main

import (
	"fmt"
	"os"
	"strings"
)

var (
	flagFiles        []string
	flagRegistryDirs []string
	flagDryRun       bool
)

func main() {
	rootCmd.AddCommand(listCmd)

	rootCmd.Flags().StringArrayVarP(&flagFiles, "file", "f", nil,
		"node YAML file (repeatable; default: ~/.config/"+appName+"/nodes/*.yml)")
	rootCmd.Flags().StringArrayVar(&flagRegistryDirs, "registry-dir", nil,
		"additional registry directory to scan for type files (repeatable)")
	rootCmd.Flags().BoolVar(&flagDryRun, "dry-run", false,
		"print what would be executed without running anything")

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		if isFlagInterceptError(err) {
			fmt.Fprintln(os.Stderr, "\nhint: flags after the command path are intercepted by "+appName+"; use -- to pass them through:")
			fmt.Fprintln(os.Stderr, "  "+appName+" [path] -- <flags>")
			fmt.Fprintln(os.Stderr, "  example: "+appName+" gene infra down -- -v")
		}
		os.Exit(1)
	}
}

// isFlagInterceptError reports whether the error is cobra intercepting a flag
// that was meant for the underlying command.
func isFlagInterceptError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unknown flag:") || strings.Contains(msg, "unknown shorthand flag:")
}
