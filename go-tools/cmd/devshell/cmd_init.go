package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const initRegistryHeader = "# devshell registry — type definitions\n" +
	"# ─────────────────────────────────────────────────────────────────────────────\n" +
	"# Define reusable types here. Types are referenced via `uses` in node files.\n" +
	"# Quick reference:  devshell example\n" +
	"# Full docs:        devshell example --full\n" +
	"# ─────────────────────────────────────────────────────────────────────────────\n\n"

const initNodesHeader = `# devshell nodes
# ─────────────────────────────────────────────────────────────────────────────
# Define your commands here. Types from the registry/ directory are available.
# Quick reference:  devshell example
# Full docs:        devshell example --full
# ─────────────────────────────────────────────────────────────────────────────

`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise the devshell config directory with example files",
	Long: "Create the devshell config directory structure and populate it with\n" +
		"starter files. By default the quick-reference examples are used.\n" +
		"Use --full for the fully annotated versions.\n\n" +
		"Directories created:\n" +
		"  <config>/registry/   — type definitions\n" +
		"  <config>/nodes/      — node definitions\n\n" +
		"The default config directory follows the same priority as the main command:\n" +
		"  $DEVSHELL_CONFIG_DIR > $XDG_CONFIG_HOME/devshell > ~/.config/devshell",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		force, _ := cmd.Flags().GetBool("force")
		dir, _ := cmd.Flags().GetString("dir")

		if dir == "" {
			var err error
			dir, err = resolveConfigDir()
			if err != nil {
				return err
			}
		}

		var typesYAML, nodesYAML []byte
		if full {
			typesYAML = exampleFullTypesYAML
			nodesYAML = exampleFullNodesYAML
		} else {
			typesYAML = exampleSimpleTypesYAML
			nodesYAML = exampleSimpleNodesYAML
		}

		registryDir := filepath.Join(dir, "registry")
		nodesDir := filepath.Join(dir, "nodes")

		for _, d := range []string{registryDir, nodesDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", d, err)
			}
		}

		registryFile := filepath.Join(registryDir, "types.yml")
		nodesFile := filepath.Join(nodesDir, "nodes.yml")

		if err := writeInitFile(registryFile, initRegistryHeader, typesYAML, force); err != nil {
			return err
		}
		if err := writeInitFile(nodesFile, initNodesHeader, nodesYAML, force); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "initialised %s\n", dir)
		fmt.Fprintf(os.Stderr, "  %s\n", registryFile)
		fmt.Fprintf(os.Stderr, "  %s\n", nodesFile)
		fmt.Fprintf(os.Stderr, "\nRun `devshell list` to see available commands.\n")
		return nil
	},
}

func writeInitFile(path, header string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()
	fmt.Fprint(f, header)
	_, err = f.Write(content)
	return err
}

func init() {
	initCmd.Flags().Bool("full", false, "use the full annotated examples instead of the quick reference")
	initCmd.Flags().Bool("force", false, "overwrite existing files")
	initCmd.Flags().String("dir", "", "target config directory (default: auto-resolved)")
}
