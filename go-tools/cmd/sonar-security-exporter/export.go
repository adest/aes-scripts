package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	var (
		project     string
		branch      string
		pullRequest string
		tags        []string
		output      string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export security vulnerabilities and hotspots to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if branch != "" && pullRequest != "" {
				return fmt.Errorf("--branch and --pull-request are mutually exclusive")
			}

			token, err := resolveToken(flagToken)
			if err != nil {
				return err
			}
			client := NewClient(flagURL, token)

			fmt.Fprintln(os.Stderr, "Fetching vulnerabilities...")
			vulns, err := client.FetchVulnerabilities(project, branch, pullRequest, tags)
			if err != nil {
				return fmt.Errorf("fetch vulnerabilities: %w", err)
			}

			fmt.Fprintln(os.Stderr, "Fetching security hotspots...")
			hotspots, err := client.FetchHotspots(project, branch, pullRequest)
			if err != nil {
				return fmt.Errorf("fetch hotspots: %w", err)
			}

			// Use empty slices instead of nil so the JSON output is [] not null.
			if vulns == nil {
				vulns = []Issue{}
			}
			if hotspots == nil {
				hotspots = []Hotspot{}
			}

			result := ExportResult{
				Project:              project,
				Organization:         flagOrg,
				ExportedAt:           time.Now().UTC().Format(time.RFC3339),
				Branch:               branch,
				PullRequest:          pullRequest,
				TotalVulnerabilities: len(vulns),
				TotalHotspots:        len(hotspots),
				Vulnerabilities:      vulns,
				Hotspots:             hotspots,
			}

			var data []byte
			if flagPretty {
				data, err = json.MarshalIndent(result, "", "  ")
			} else {
				data, err = json.Marshal(result)
			}
			if err != nil {
				return err
			}
			data = append(data, '\n')

			if output == "" || output == "-" {
				_, err = os.Stdout.Write(data)
				return err
			}
			if err := os.WriteFile(output, data, 0644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Written to %s (%d vulnerabilities, %d hotspots)\n",
				output, len(vulns), len(hotspots))
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project key (required)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name")
	cmd.Flags().StringVar(&pullRequest, "pull-request", "", "Pull request number")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Filter vulnerabilities by issue tags, comma-separated (e.g. security,owasp-a1)")
	cmd.Flags().StringVarP(&output, "output", "f", "", "Output file path (default: stdout)")
	_ = cmd.MarkFlagRequired("project")

	return cmd
}
