package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newListProjectsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-projects",
		Short: "List projects available in the organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagOrg == "" {
				return fmt.Errorf("--organization is required for list-projects")
			}
			token, err := resolveToken(flagToken)
			if err != nil {
				return err
			}
			client := NewClient(flagURL, token)

			projects, err := client.FetchProjects(flagOrg)
			if err != nil {
				return err
			}

			var data []byte
			if flagPretty {
				data, err = json.MarshalIndent(projects, "", "  ")
			} else {
				data, err = json.Marshal(projects)
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
			return err
		},
	}
	return cmd
}
