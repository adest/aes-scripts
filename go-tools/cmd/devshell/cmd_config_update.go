package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const examplesStartMarker = "# @devshell-examples-start"
const examplesEndMarker = "# @devshell-examples-end"

var configUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Refresh commented examples and spec in an existing config directory",
	Long: "Update the commented example blocks (between @devshell-examples-start and\n" +
		"@devshell-examples-end markers) in registry/types.yml and nodes/nodes.yml,\n" +
		"and refresh spec/dsl-spec.md.\n\n" +
		"Only files that contain the sentinel markers are modified.\n" +
		"Files without markers are left untouched.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			var err error
			dir, err = resolveConfigDir()
			if err != nil {
				return err
			}
		}

		registryFile := filepath.Join(dir, "registry", "types.yml")
		nodesFile := filepath.Join(dir, "nodes", "nodes.yml")
		specFile := filepath.Join(dir, "spec", "dsl-spec.md")

		newRegistryBlock := extractExampleBlock(initRegistryYAML)
		newNodesBlock := extractExampleBlock(initNodesYAML)

		updated := 0

		changed, err := updateExampleBlock(registryFile, newRegistryBlock)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(os.Stderr, "updated %s\n", registryFile)
			updated++
		}

		changed, err = updateExampleBlock(nodesFile, newNodesBlock)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(os.Stderr, "updated %s\n", nodesFile)
			updated++
		}

		// Always refresh spec silently if it changed
		specDir := filepath.Join(dir, "spec")
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", specDir, err)
		}
		existingSpec, _ := os.ReadFile(specFile)
		if !bytes.Equal(existingSpec, dslSpecMD) {
			if err := os.WriteFile(specFile, dslSpecMD, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", specFile, err)
			}
			fmt.Fprintf(os.Stderr, "updated %s\n", specFile)
			updated++
		}

		if updated == 0 {
			fmt.Fprintln(os.Stderr, "everything up to date")
		}
		return nil
	},
}

// extractExampleBlock extracts the lines from the start marker to the end marker
// (inclusive) from content. Returns nil if the markers are not found.
func extractExampleBlock(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	var result [][]byte
	inBlock := false
	for _, line := range lines {
		trimmed := bytes.TrimRight(line, " \t")
		if bytes.Equal(trimmed, []byte(examplesStartMarker)) {
			inBlock = true
		}
		if inBlock {
			result = append(result, line)
		}
		if inBlock && bytes.Equal(trimmed, []byte(examplesEndMarker)) {
			break
		}
	}
	if len(result) == 0 {
		return nil
	}
	return bytes.Join(result, []byte("\n"))
}

// updateExampleBlock replaces the sentinel block in the file at path with newBlock.
// Returns true if the file was modified.
func updateExampleBlock(path string, newBlock []byte) (bool, error) {
	if newBlock == nil {
		return false, nil
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading %s: %w", path, err)
	}
	updated, changed, err := replaceExampleBlock(existing, newBlock)
	if err != nil || !changed {
		return false, err
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, nil
}

// replaceExampleBlock replaces the content between (and including) the sentinel
// markers in content with newBlock. Returns the modified content and whether a
// change was made.
func replaceExampleBlock(content, newBlock []byte) ([]byte, bool, error) {
	lines := bytes.Split(content, []byte("\n"))
	startIdx, endIdx := -1, -1
	for i, line := range lines {
		trimmed := bytes.TrimRight(line, " \t")
		if bytes.Equal(trimmed, []byte(examplesStartMarker)) {
			startIdx = i
		}
		if bytes.Equal(trimmed, []byte(examplesEndMarker)) {
			endIdx = i
			break
		}
	}
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return content, false, nil
	}

	// Check if the block already matches
	existingBlock := bytes.Join(lines[startIdx:endIdx+1], []byte("\n"))
	if bytes.Equal(existingBlock, newBlock) {
		return content, false, nil
	}

	var out [][]byte
	out = append(out, lines[:startIdx]...)
	out = append(out, bytes.Split(newBlock, []byte("\n"))...)
	out = append(out, lines[endIdx+1:]...)
	return bytes.Join(out, []byte("\n")), true, nil
}

func init() {
	configUpdateCmd.Flags().String("dir", "", "target config directory (default: auto-resolved)")
}
