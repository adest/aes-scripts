package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	flagTCP   bool
	flagUDP   bool
	flagAll   bool
	flagNoTUI bool
)

var rootCmd = &cobra.Command{
	Use:   "tcpo",
	Short: "Visualise et gère les ports en écoute",
	Long: `tcpo liste les ports en écoute (TCP par défaut) et permet de kill
les processus associés via une interface interactive.`,
	RunE: run,
	// tcpo takes no positional arguments, only flags.
	// Inject "--" when the cursor is empty so Cobra triggers its native flag
	// completion instead of falling back to file completion.
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 && toComplete == "" {
			return []string{"--"}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	rootCmd.Flags().BoolVar(&flagTCP, "tcp", false, "Scanner les ports TCP (défaut si aucun flag)")
	rootCmd.Flags().BoolVar(&flagUDP, "udp", false, "Scanner les ports UDP")
	rootCmd.Flags().BoolVar(&flagAll, "all", false, "Scanner TCP + UDP")
	rootCmd.Flags().BoolVar(&flagNoTUI, "no-tui", false, "Affichage texte simple sans interface interactive")
}

func main() {
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	proto := resolveProto()

	connections, err := ScanConnections(proto)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	if flagNoTUI {
		printConnections(connections)
		return nil
	}

	p := tea.NewProgram(newModel(connections, proto), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func resolveProto() string {
	if flagAll {
		return "all"
	}
	if flagUDP {
		return "udp"
	}
	return "tcp"
}

func printConnections(connections []ConnectionInfo) {
	fmt.Printf("%-6s %-22s %-8s %-20s\n", "PROTO", "ADRESSE", "PID", "PROCESSUS")
	fmt.Println(strings.Repeat("-", 60))
	for _, c := range connections {
		fmt.Printf("%-6s %-22s %-8d %-20s\n", c.Proto, c.LocalAddr, c.Pid, c.ProcessName)
	}
}
