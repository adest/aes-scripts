package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed cmd_example_simple.yml
var exampleSimpleYAML []byte

//go:embed cmd_example.yml
var exampleFullYAML []byte

var exampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Print a reference configuration covering all DSL features",
	Long: "Print a devshell YAML configuration that demonstrates every DSL feature.\n" +
		"By default a concise quick-reference is printed. Use --full for the annotated\n" +
		"complete example. Use --output to write to a file instead of stdout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		data := exampleSimpleYAML
		if full {
			data = exampleFullYAML
		}

		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			os.Stdout.Write(data)
			return nil
		}
		if err := os.WriteFile(output, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", output, err)
		}
		fmt.Fprintf(os.Stderr, "written to %s\n", output)
		return nil
	},
}

func init() {
	exampleCmd.Flags().StringP("output", "o", "", "write to file instead of stdout")
	exampleCmd.Flags().Bool("full", false, "print the full annotated example instead of the quick reference")
}
