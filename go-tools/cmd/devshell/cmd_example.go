package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed cmd_example_simple_types.yml
var exampleSimpleTypesYAML []byte

//go:embed cmd_example_simple_nodes.yml
var exampleSimpleNodesYAML []byte

//go:embed cmd_example_full_types.yml
var exampleFullTypesYAML []byte

//go:embed cmd_example_full_nodes.yml
var exampleFullNodesYAML []byte

const exampleSimpleHeader = `# devshell — quick reference
# Run:          devshell --file <this-file> <command>
# Full example: devshell example --full

`

const exampleFullHeader = `# devshell — full reference
# ─────────────────────────────────────────────────────────────────────────────
# This file documents every feature of the devshell DSL.
# Run it with:     devshell --file <this-file> <command>
# Preview it with: devshell --file <this-file> --dry-run <command>
# ─────────────────────────────────────────────────────────────────────────────

`

var exampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Print a reference configuration covering all DSL features",
	Long: "Print a devshell YAML configuration that demonstrates every DSL feature.\n" +
		"By default a concise quick-reference is printed. Use --full for the annotated\n" +
		"complete example. Use --output to write to a file instead of stdout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")

		var header string
		var typesYAML, nodesYAML []byte
		if full {
			header = exampleFullHeader
			typesYAML = exampleFullTypesYAML
			nodesYAML = exampleFullNodesYAML
		} else {
			header = exampleSimpleHeader
			typesYAML = exampleSimpleTypesYAML
			nodesYAML = exampleSimpleNodesYAML
		}

		output, _ := cmd.Flags().GetString("output")
		w := os.Stdout
		if output != "" {
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("creating %s: %w", output, err)
			}
			defer f.Close()
			w = f
		}

		fmt.Fprint(w, header)
		w.Write(typesYAML)
		fmt.Fprintln(w)
		w.Write(nodesYAML)

		if output != "" {
			fmt.Fprintf(os.Stderr, "written to %s\n", output)
		}
		return nil
	},
}

func init() {
	exampleCmd.Flags().StringP("output", "o", "", "write to file instead of stdout")
	exampleCmd.Flags().Bool("full", false, "print the full annotated example instead of the quick reference")
}
