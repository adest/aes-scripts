package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed cmd_config_init_registry.yml
var initRegistryYAML []byte

//go:embed cmd_config_init_nodes.yml
var initNodesYAML []byte

//go:embed doc/dsl-spec.md
var dslSpecMD []byte

const configInitRegistryHeader = "# devshell registry — type definitions\n" +
	"# ─────────────────────────────────────────────────────────────────────────────\n" +
	"# Define reusable types here. Types are referenced via `uses` in node files.\n" +
	"# Quick reference:  devshell example\n" +
	"# Full docs:        devshell example --full\n" +
	"# ─────────────────────────────────────────────────────────────────────────────\n\n"

const configInitNodesHeader = "# devshell nodes\n" +
	"# ─────────────────────────────────────────────────────────────────────────────\n" +
	"# Define your commands here. Types from the registry/ directory are available.\n" +
	"# Quick reference:  devshell example\n" +
	"# Full docs:        devshell example --full\n" +
	"# ─────────────────────────────────────────────────────────────────────────────\n\n"

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise the devshell config directory with starter files",
	Long: "Create the devshell config directory structure and populate it with\n" +
		"starter files. A single concrete `hello` command is created so the shell\n" +
		"is immediately usable. All other examples are commented out behind sentinel\n" +
		"markers and can be refreshed later with `devshell config update`.\n\n" +
		"Directories created:\n" +
		"  <config>/registry/   — type definitions\n" +
		"  <config>/nodes/      — node definitions\n" +
		"  <config>/spec/       — DSL specification (dsl-spec.md)\n\n" +
		"The default config directory follows the same priority as the main command:\n" +
		"  $DEVSHELL_CONFIG_DIR > $XDG_CONFIG_HOME/devshell > ~/.config/devshell",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		dir, _ := cmd.Flags().GetString("dir")

		if dir == "" {
			var err error
			dir, err = resolveConfigDir()
			if err != nil {
				return err
			}
		}

		registryDir := filepath.Join(dir, "registry")
		nodesDir := filepath.Join(dir, "nodes")
		specDir := filepath.Join(dir, "spec")

		for _, d := range []string{registryDir, nodesDir, specDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", d, err)
			}
		}

		registryFile := filepath.Join(registryDir, "types.yml")
		nodesFile := filepath.Join(nodesDir, "nodes.yml")
		specFile := filepath.Join(specDir, "dsl-spec.md")

		if err := writeInitFile(registryFile, configInitRegistryHeader, initRegistryYAML, force); err != nil {
			return err
		}
		if err := writeInitFile(nodesFile, configInitNodesHeader, initNodesYAML, force); err != nil {
			return err
		}
		if err := writeInitFile(specFile, "", dslSpecMD, force); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "initialised %s\n", dir)
		fmt.Fprintf(os.Stderr, "  %s\n", registryFile)
		fmt.Fprintf(os.Stderr, "  %s\n", nodesFile)
		fmt.Fprintf(os.Stderr, "  %s\n", specFile)
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
	if header != "" {
		fmt.Fprint(f, header)
	}
	_, err = f.Write(content)
	return err
}

func init() {
	configInitCmd.Flags().Bool("force", false, "overwrite existing files")
	configInitCmd.Flags().String("dir", "", "target config directory (default: auto-resolved)")
}
