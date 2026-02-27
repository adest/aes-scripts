package main

import (
	"os"
	"fmt"
	"path/filepath"
	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	var allFlag bool
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up go-tools binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd, args, allFlag)
		},
	}
	cmd.Flags().BoolVar(&allFlag, "all", false, "Delete all binaries, including go-tools itself")
	return cmd
}

func runClean(cmd *cobra.Command, args []string, all bool) error {
	targetDir := goToolsBinDir()
	fmt.Println("Deleting binaries in", targetDir)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !all && name == "go-tools" {
			continue
		}
		path := filepath.Join(targetDir, name)
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", path, err)
		} else {
			fmt.Println("Deleted:", path)
		}
	}
	fmt.Println("Cleaning done.")
	return nil
}