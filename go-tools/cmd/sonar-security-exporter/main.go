package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Shared flags, bound to the root persistent flags so all subcommands inherit them.
var (
	flagToken  string
	flagOrg    string
	flagURL    string
	flagPretty bool
)

func main() {
	root := &cobra.Command{
		Use:   "sonar-security-exporter",
		Short: "Export SonarCloud security issues to JSON",
	}

	root.PersistentFlags().StringVar(&flagToken, "token", "", "SonarCloud token (prefer SONAR_TOKEN env var)")
	root.PersistentFlags().StringVarP(&flagOrg, "organization", "o", "", "SonarCloud organization key")
	root.PersistentFlags().StringVar(&flagURL, "url", "https://sonarcloud.io", "SonarCloud base URL")
	root.PersistentFlags().BoolVar(&flagPretty, "pretty", false, "Pretty-print JSON output")

	root.AddCommand(newExportCommand())
	root.AddCommand(newListProjectsCommand())
	root.AddCommand(newSummaryCommand())

	root.SilenceErrors = true
	root.SilenceUsage = true

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// resolveToken returns the token from the SONAR_TOKEN environment variable,
// falling back to the --token flag with a warning (flag values are visible
// in process listings and shell history).
func resolveToken(flagVal string) (string, error) {
	if t := os.Getenv("SONAR_TOKEN"); t != "" {
		return t, nil
	}
	if flagVal != "" {
		fmt.Fprintln(os.Stderr, "warning: --token is visible in the process list and shell history; prefer the SONAR_TOKEN environment variable")
		return flagVal, nil
	}
	return "", fmt.Errorf("no token: set the SONAR_TOKEN environment variable or use --token")
}
