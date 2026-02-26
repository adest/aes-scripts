package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-tools/cmd/devshell/dsl"
	"go-tools/cmd/devshell/dslyaml"
)

// appName is the single source of truth for the application name.
// All derived identifiers (env vars, config paths, error messages) are computed from it.
const appName = "devshell"

// Derived env var names — computed once at init from appName.
var (
	envConfigDir    = strings.ToUpper(appName) + "_CONFIG_DIR"
	envRegistryDirs = strings.ToUpper(appName) + "_REGISTRY_DIRS"
	envNodes        = strings.ToUpper(appName) + "_NODES"
)

// resolveConfigDir returns the base config directory for the application.
// Priority: $<APPNAME>_CONFIG_DIR > $XDG_CONFIG_HOME/<appName> > ~/.config/<appName>
func resolveConfigDir() (string, error) {
	if v := os.Getenv(envConfigDir); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", appName), nil
}

// resolveRegistryDirs returns all registry directories to scan for type files.
// Order: configDir/registry → $<APPNAME>_REGISTRY_DIRS → flagDirs
func resolveRegistryDirs(configDir string, flagDirs []string) []string {
	dirs := []string{filepath.Join(configDir, "registry")}
	dirs = append(dirs, splitColon(os.Getenv(envRegistryDirs))...)
	dirs = append(dirs, flagDirs...)
	return dirs
}

// resolveNodeFiles returns all node files to load.
// Order: configDir/nodes/*.yml → $<APPNAME>_NODES → flagFiles
// Missing directories are silently skipped; explicitly provided paths are kept as-is
// (errors will surface at read time with a clear message).
func resolveNodeFiles(configDir string, flagFiles []string) ([]string, error) {
	autoFiles, err := globYAML(filepath.Join(configDir, "nodes"))
	if err != nil {
		return nil, err
	}
	files := autoFiles
	files = append(files, splitColon(os.Getenv(envNodes))...)
	files = append(files, flagFiles...)
	return files, nil
}

// globYAML returns sorted *.yml / *.yaml files in dir.
// Returns nil without error if dir does not exist.
func globYAML(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

// splitColon splits a colon-separated string, filtering empty parts.
func splitColon(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadSources reads all registry and node files and builds the runtime tree.
// Registry files are loaded first so their types are available to all node files.
func loadSources(registryDirs, nodeFiles []string) (*dsl.Container, error) {
	if len(nodeFiles) == 0 {
		return nil, fmt.Errorf(
			"no node files found: add *.yml files to ~/.config/%s/nodes/, "+
				"set $%s, or use --file",
			appName, envNodes,
		)
	}

	var inputs [][]byte

	for _, dir := range registryDirs {
		files, err := globYAML(dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, fmt.Errorf("registry file %s: %w", f, err)
			}
			inputs = append(inputs, data)
		}
	}

	for _, f := range nodeFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("node file %s: %w", f, err)
		}
		inputs = append(inputs, data)
	}

	return dslyaml.BuildMany(inputs...)
}
