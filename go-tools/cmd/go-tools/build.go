package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type platform struct {
	GOOS   string
	GOARCH string
	Label  string
}

// Add new platforms here.
var allPlatforms = []platform{
	{GOOS: "windows", GOARCH: "amd64", Label: "windows-x86"},
	{GOOS: "linux", GOARCH: "amd64", Label: "linux-x86"},
	{GOOS: "darwin", GOARCH: "amd64", Label: "mac-x86"},
	{GOOS: "darwin", GOARCH: "arm64", Label: "mac-arm"},
}

func newBuildCommand() *cobra.Command {
	var platformFlags []string
	var appFlags []string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Cross-compile binaries into dist/<platform>/",
		Long: fmt.Sprintf(`Cross-compile go-tools binaries for multiple platforms.

Binaries are written to dist/<platform>/ in the repository root.

Available platforms: %s`, platformLabels()),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(platformFlags, appFlags)
		},
	}

	cmd.Flags().StringArrayVar(&platformFlags, "platform", nil,
		fmt.Sprintf("Target platform(s) to build (available: %s)", platformLabels()))
	cmd.Flags().StringArrayVar(&appFlags, "app", nil,
		"App(s) to build (default: all apps in cmd/)")

	return cmd
}

func runBuild(platformFlags []string, appFlags []string) error {
	repoRoot := getRepoRoot()

	targets, err := resolvePlatforms(platformFlags)
	if err != nil {
		return err
	}

	apps, err := resolveApps(repoRoot, appFlags)
	if err != nil {
		return err
	}

	for _, p := range targets {
		outDir := filepath.Join(repoRoot, "dist", p.Label)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create output dir %q: %w", outDir, err)
		}

		for _, app := range apps {
			binaryName := app
			if p.GOOS == "windows" {
				binaryName += ".exe"
			}
			outPath := filepath.Join(outDir, binaryName)

			fmt.Printf("→ Building %-20s [%s]\n", app, p.Label)

			build := exec.Command("go", "build", "-o", outPath, "./cmd/"+app)
			build.Dir = repoRoot
			build.Env = append(os.Environ(),
				"GOOS="+p.GOOS,
				"GOARCH="+p.GOARCH,
				"CGO_ENABLED=0",
			)
			build.Stdout = os.Stdout
			build.Stderr = os.Stderr

			if err := build.Run(); err != nil {
				repro := fmt.Sprintf("cd %s && GOOS=%s GOARCH=%s CGO_ENABLED=0 %s",
					repoRoot, p.GOOS, p.GOARCH, strings.Join(build.Args, " "))
				return fmt.Errorf("build failed for %q on %q\ncommand: %s\nerror: %w",
					app, p.Label, repro, err)
			}
		}
	}

	fmt.Printf("✅ Binaries written to %s\n", filepath.Join(repoRoot, "dist"))
	return nil
}

func resolvePlatforms(flags []string) ([]platform, error) {
	if len(flags) == 0 {
		return allPlatforms, nil
	}
	var result []platform
	for _, label := range flags {
		p, ok := findPlatform(label)
		if !ok {
			return nil, fmt.Errorf("unknown platform %q (available: %s)", label, platformLabels())
		}
		result = append(result, p)
	}
	return result, nil
}

func resolveApps(repoRoot string, flags []string) ([]string, error) {
	if len(flags) > 0 {
		for _, app := range flags {
			appPath := filepath.Join(repoRoot, "cmd", app)
			if _, err := os.Stat(appPath); os.IsNotExist(err) {
				return nil, fmt.Errorf("unknown app %q (no directory at cmd/%s)", app, app)
			}
		}
		return flags, nil
	}

	cmdDir := filepath.Join(repoRoot, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, err
	}

	var apps []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "go-tools" {
			apps = append(apps, e.Name())
		}
	}
	return apps, nil
}

func findPlatform(label string) (platform, bool) {
	for _, p := range allPlatforms {
		if p.Label == label {
			return p, true
		}
	}
	return platform{}, false
}

func platformLabels() string {
	labels := make([]string, len(allPlatforms))
	for i, p := range allPlatforms {
		labels[i] = p.Label
	}
	return strings.Join(labels, ", ")
}
