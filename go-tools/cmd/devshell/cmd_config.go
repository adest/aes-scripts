package main

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the devshell config directory",
	Long:  "Commands for initialising and updating the devshell config directory.",
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configUpdateCmd)
}
