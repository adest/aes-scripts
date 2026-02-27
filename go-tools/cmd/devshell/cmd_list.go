package main

import (
	"fmt"
	"strings"

	"go-tools/cmd/devshell/dsl"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all executable commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := load(flagFiles, flagRegistryDirs)
		if err != nil {
			return err
		}
		printLeaves(collectLeaves(root, nil))
		return nil
	},
}

// leafEntry holds the full invocation path and kind of an executable leaf node.
type leafEntry struct {
	path string // space-joined path segments, e.g. "backend build"
	kind string // "runnable" or "pipeline"
}

// collectLeaves recursively walks the tree and returns all executable leaf nodes.
// prefix accumulates the path segments from the root down to the current node.
func collectLeaves(node dsl.Node, prefix []string) []leafEntry {
	switch n := node.(type) {
	case *dsl.Runnable:
		return []leafEntry{{path: strings.Join(prefix, " "), kind: "runnable"}}
	case *dsl.Pipeline:
		return []leafEntry{{path: strings.Join(prefix, " "), kind: "pipeline"}}
	case *dsl.Container:
		var out []leafEntry
		for _, child := range n.Children {
			out = append(out, collectLeaves(child, append(prefix, child.Name()))...)
		}
		return out
	}
	return nil
}

// printLeaves prints all leaf entries aligned, prefixed by appName.
func printLeaves(entries []leafEntry) {
	if len(entries) == 0 {
		fmt.Println("no commands found")
		return
	}

	maxLen := 0
	for _, e := range entries {
		if n := len(appName) + 1 + len(e.path); n > maxLen {
			maxLen = n
		}
	}

	for _, e := range entries {
		full := appName + " " + e.path
		fmt.Printf("%-*s  [%s]\n", maxLen, full, e.kind)
	}
}
