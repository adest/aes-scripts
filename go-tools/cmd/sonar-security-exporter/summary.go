package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSummaryCommand() *cobra.Command {
	var (
		project     string
		branch      string
		pullRequest string
	)

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Print a human-readable security summary for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if branch != "" && pullRequest != "" {
				return fmt.Errorf("--branch and --pull-request are mutually exclusive")
			}

			token, err := resolveToken(flagToken)
			if err != nil {
				return err
			}
			client := NewClient(flagURL, token)

			vulns, err := client.FetchVulnerabilities(project, branch, pullRequest, nil)
			if err != nil {
				return fmt.Errorf("fetch vulnerabilities: %w", err)
			}
			hotspots, err := client.FetchHotspots(project, branch, pullRequest)
			if err != nil {
				return fmt.Errorf("fetch hotspots: %w", err)
			}

			// Header
			fmt.Printf("Project:  %s\n", project)
			if flagOrg != "" {
				fmt.Printf("Org:      %s\n", flagOrg)
			}
			if branch != "" {
				fmt.Printf("Branch:   %s\n", branch)
			}
			if pullRequest != "" {
				fmt.Printf("PR:       #%s\n", pullRequest)
			}
			fmt.Println()

			// Vulnerabilities by severity
			fmt.Printf("Vulnerabilities: %d\n", len(vulns))
			bySeverity := map[string]int{}
			for _, v := range vulns {
				bySeverity[v.Severity]++
			}
			for _, sev := range []string{"BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"} {
				if n := bySeverity[sev]; n > 0 {
					fmt.Printf("  %-10s %d\n", sev, n)
				}
			}
			fmt.Println()

			// Hotspots by vulnerability probability
			fmt.Printf("Security Hotspots: %d\n", len(hotspots))
			byProb := map[string]int{}
			for _, h := range hotspots {
				byProb[h.VulnerabilityProbability]++
			}
			for _, prob := range []string{"HIGH", "MEDIUM", "LOW"} {
				if n := byProb[prob]; n > 0 {
					fmt.Printf("  %-10s %d\n", prob, n)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project key (required)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name")
	cmd.Flags().StringVar(&pullRequest, "pull-request", "", "Pull request number")
	_ = cmd.MarkFlagRequired("project")

	return cmd
}
