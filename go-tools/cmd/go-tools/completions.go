package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const completionsMarkerBegin = "# >>> go-tools completions >>>"
const completionsMarkerEnd = "# <<< go-tools completions <<<"

// shellDef describes how to install and source completions for a given shell.
type shellDef struct {
	name           string
	configFilePath string
	// setupBlock returns the lines to add between the markers.
	setupBlock func(completionsDir string) string
	// fileName returns the completion file name for a given tool.
	fileName func(tool string) string
}

func allShells() []shellDef {
	home := os.Getenv("HOME")
	return []shellDef{
		{
			name:           "zsh",
			configFilePath: filepath.Join(home, ".zshrc"),
			setupBlock: func(dir string) string {
				return "fpath=(" + dir + " $fpath)\nautoload -U compinit && compinit"
			},
			fileName: func(tool string) string { return "_" + tool },
		},
		{
			name:           "bash",
			configFilePath: filepath.Join(home, ".bashrc"),
			setupBlock: func(dir string) string {
				return `for f in ` + dir + `/*; do [ -f "$f" ] && source "$f"; done`
			},
			fileName: func(tool string) string { return tool },
		},
		{
			name:           "fish",
			configFilePath: filepath.Join(home, ".config", "fish", "config.fish"),
			setupBlock: func(dir string) string {
				return "for f in " + dir + "/*.fish; source $f; end"
			},
			fileName: func(tool string) string { return tool + ".fish" },
		},
	}
}

func detectShell() *shellDef {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return nil
	}
	name := filepath.Base(shellPath)
	for _, s := range allShells() {
		if s.name == name {
			sc := s
			return &sc
		}
	}
	return nil
}

func resolveShell(name string) (*shellDef, error) {
	if name != "" {
		for _, s := range allShells() {
			if s.name == name {
				sc := s
				return &sc, nil
			}
		}
		return nil, fmt.Errorf("unsupported shell %q (supported: zsh, bash, fish)", name)
	}
	s := detectShell()
	if s == nil {
		return nil, fmt.Errorf("could not detect shell from $SHELL; use --shell <zsh|bash|fish>")
	}
	return s, nil
}

// generateCompletionForTool runs `<binary> completion <shell>` and returns the output.
// The subprocess runs in a new session (Setsid) so it has no controlling terminal:
// interactive TUI tools (e.g. fuzzyfinder-based) that open /dev/tty directly will fail
// immediately instead of corrupting the terminal state.
func generateCompletionForTool(binPath, shell string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "completion", shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil, false
	}
	// All shell completion scripts start with '#'. Anything else is garbage output
	// from a tool that doesn't support completions (e.g. a TUI that partially ran).
	if out[0] != '#' {
		return nil, false
	}
	return out, true
}

// completionInstallResult holds the outcome of a completion install run.
type completionInstallResult struct {
	installed []string // completion generated and written
	excluded  []string // in completionExcludeList, intentionally skipped
	failed    []string // tried but got no valid completion output
}

func isExcluded(name string) bool {
	for _, e := range completionExcludeList {
		if e == name {
			return true
		}
	}
	return false
}

// installCompletionsForShell generates and writes completion scripts for all
// installed tools, skipping those in completionExcludeList.
func installCompletionsForShell(shell shellDef) (completionInstallResult, error) {
	var res completionInstallResult

	binDir := goToolsBinDir()
	entries, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return res, fmt.Errorf("bin directory not found at %s: run 'go-tools install' first", binDir)
		}
		return res, err
	}

	compDir := completionsDirFor(shell.name)
	if err := os.MkdirAll(compDir, 0755); err != nil {
		return res, err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		toolName := e.Name()

		if isExcluded(toolName) {
			res.excluded = append(res.excluded, toolName)
			continue
		}

		binPath := filepath.Join(binDir, toolName)
		content, ok := generateCompletionForTool(binPath, shell.name)
		if !ok {
			res.failed = append(res.failed, toolName)
			continue
		}

		filePath := filepath.Join(compDir, shell.fileName(toolName))
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return res, fmt.Errorf("failed to write completion for %s: %w", toolName, err)
		}
		res.installed = append(res.installed, toolName)
	}
	return res, nil
}

// isShellConfigured reports whether the shell config file contains the go-tools block.
func isShellConfigured(shell shellDef) (bool, error) {
	f, err := os.Open(shell.configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), completionsMarkerBegin) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// configureShell appends the go-tools completions block to the shell config file.
func configureShell(shell shellDef) error {
	compDir := completionsDirFor(shell.name)
	block := completionsMarkerBegin + "\n" + shell.setupBlock(compDir) + "\n" + completionsMarkerEnd + "\n"

	if err := os.MkdirAll(filepath.Dir(shell.configFilePath), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(shell.configFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + block)
	return err
}

// unconfigureShell removes the go-tools completions block from the shell config file.
func unconfigureShell(shell shellDef) error {
	content, err := os.ReadFile(shell.configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inBlock := false
	for _, line := range lines {
		if strings.Contains(line, completionsMarkerBegin) {
			inBlock = true
			continue
		}
		if strings.Contains(line, completionsMarkerEnd) {
			inBlock = false
			continue
		}
		if !inBlock {
			result = append(result, line)
		}
	}
	return os.WriteFile(shell.configFilePath, []byte(strings.Join(result, "\n")), 0644)
}

// --- Commands ---

func newCompletionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completions",
		Short: "Manage shell completions for go-tools",
	}
	cmd.AddCommand(newCompletionsInstallCommand())
	cmd.AddCommand(newCompletionsSetupCommand())
	cmd.AddCommand(newCompletionsStatusCommand())
	cmd.AddCommand(newCompletionsCleanCommand())
	return cmd
}

func newCompletionsInstallCommand() *cobra.Command {
	var shellFlag string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate and write completion scripts for the current shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletionsInstall(shellFlag)
		},
	}
	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell to install completions for (default: auto-detect)")
	return cmd
}

func runCompletionsInstall(shellName string) error {
	shell, err := resolveShell(shellName)
	if err != nil {
		return err
	}

	fmt.Printf("→ Generating completions for %s\n", shell.name)
	res, err := installCompletionsForShell(*shell)
	if err != nil {
		return err
	}
	printCompletionResult(res)
	fmt.Printf("Completions written to %s\n", completionsDirFor(shell.name))

	configured, err := isShellConfigured(*shell)
	if err != nil {
		return err
	}
	if !configured {
		fmt.Printf("\n⚠️  Your shell is not configured to source these completions.\n")
		fmt.Printf("   Run: go-tools completions setup\n")
	}
	return nil
}

func printCompletionResult(res completionInstallResult) {
	for _, t := range res.installed {
		fmt.Printf("  ✅ %s\n", t)
	}
	for _, t := range res.excluded {
		fmt.Printf("  —  %s (excluded)\n", t)
	}
	for _, t := range res.failed {
		fmt.Printf("  ⚠️  %s (no completion support)\n", t)
	}
}

func newCompletionsSetupCommand() *cobra.Command {
	var shellFlag string
	var yesFlag bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure your shell to source go-tools completions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletionsSetup(shellFlag, yesFlag)
		},
	}
	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell to configure (default: auto-detect)")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func runCompletionsSetup(shellName string, yes bool) error {
	shell, err := resolveShell(shellName)
	if err != nil {
		return err
	}

	configured, err := isShellConfigured(*shell)
	if err != nil {
		return err
	}
	if configured {
		fmt.Printf("✅ %s is already configured for go-tools completions.\n", shell.configFilePath)
		return nil
	}

	compDir := completionsDirFor(shell.name)
	block := shell.setupBlock(compDir)

	fmt.Printf("Will append to %s:\n\n", shell.configFilePath)
	fmt.Printf("  %s\n", completionsMarkerBegin)
	for _, line := range strings.Split(block, "\n") {
		fmt.Printf("  %s\n", line)
	}
	fmt.Printf("  %s\n\n", completionsMarkerEnd)

	if !yes {
		fmt.Print("Proceed? [y/N] ")
		var resp string
		fmt.Scanln(&resp)
		if !strings.EqualFold(strings.TrimSpace(resp), "y") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := configureShell(*shell); err != nil {
		return err
	}
	fmt.Printf("✅ Done. Reload your shell or run:\n   source %s\n", shell.configFilePath)
	return nil
}

func newCompletionsStatusCommand() *cobra.Command {
	var shellFlag string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show completion setup status for the current shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletionsStatus(shellFlag)
		},
	}
	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell to check (default: auto-detect)")
	return cmd
}

func runCompletionsStatus(shellName string) error {
	shell, err := resolveShell(shellName)
	if err != nil {
		return err
	}

	fmt.Printf("Shell:       %s\n", shell.name)
	fmt.Printf("Config file: %s\n", shell.configFilePath)

	configured, err := isShellConfigured(*shell)
	if err != nil {
		return err
	}
	if configured {
		fmt.Printf("Configured:  ✅ yes\n")
	} else {
		fmt.Printf("Configured:  ❌ no  → run 'go-tools completions setup'\n")
	}

	fmt.Println("\nCompletion files:")
	compDir := completionsDirFor(shell.name)
	binDir := goToolsBinDir()

	entries, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  (no tools installed in", binDir, ")")
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		toolName := e.Name()
		if isExcluded(toolName) {
			fmt.Printf("  —  %-30s (excluded)\n", toolName)
			continue
		}
		filePath := filepath.Join(compDir, shell.fileName(toolName))
		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("  ✅ %-30s %s\n", toolName, filePath)
		} else {
			fmt.Printf("  ❌ %-30s (not generated — run 'go-tools completions install')\n", toolName)
		}
	}
	return nil
}

func newCompletionsCleanCommand() *cobra.Command {
	var shellFlag string
	var allFlag bool
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove generated completion scripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletionsClean(shellFlag, allFlag)
		},
	}
	cmd.Flags().StringVar(&shellFlag, "shell", "", "Shell to clean completions for (default: auto-detect)")
	cmd.Flags().BoolVar(&allFlag, "all", false, "Clean completions for all shells")
	return cmd
}

func runCompletionsClean(shellName string, all bool) error {
	if all {
		for _, s := range allShells() {
			compDir := completionsDirFor(s.name)
			if err := os.RemoveAll(compDir); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "warning: failed to clean %s: %v\n", compDir, err)
			} else {
				fmt.Println("Cleaned:", compDir)
			}
		}
		return nil
	}

	shell, err := resolveShell(shellName)
	if err != nil {
		return err
	}
	compDir := completionsDirFor(shell.name)
	if err := os.RemoveAll(compDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("Cleaned:", compDir)
	return nil
}
