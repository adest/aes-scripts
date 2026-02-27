package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed cmd_example.yml
var exampleYAML []byte

var exampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Print a reference configuration covering all DSL features",
	Long: "Print a complete devshell YAML configuration that demonstrates every DSL feature.\n" +
		"By default the output is written to stdout. Use --output to write to a file instead.",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			os.Stdout.Write(exampleYAML)
			return nil
		}
		if err := os.WriteFile(output, exampleYAML, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", output, err)
		}
		fmt.Fprintf(os.Stderr, "written to %s\n", output)
		return nil
	},
}

func init() {
	exampleCmd.Flags().StringP("output", "o", "", "write to file instead of stdout")
}
