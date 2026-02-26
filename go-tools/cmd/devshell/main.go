package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go-tools/cmd/devshell/dsl"

	"github.com/spf13/cobra"
)

func main() {
	var flagFiles []string
	var flagRegistryDirs []string

	rootCmd := &cobra.Command{
		Use:               appName + " [command ...]",
		Short:             "Dynamic " + appName + " CLI",
		Long:              "Dynamic " + appName + " CLI\n\nTasks are auto-completable via shell completion (Tab).",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return dynamicCompletion(flagFiles, flagRegistryDirs, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			root, err := load(flagFiles, flagRegistryDirs)
			if err != nil {
				return err
			}
			node, extraArgs, err := resolvePath(root, args)
			if err != nil {
				return err
			}
			switch n := node.(type) {
			case *dsl.Runnable:
				return execute(n, extraArgs)
			case *dsl.Pipeline:
				return executePipeline(n)
			default:
				return fmt.Errorf("unexpected node type %T", node)
			}
		},
	}

	rootCmd.Flags().StringArrayVarP(&flagFiles, "file", "f", nil,
		"node YAML file (repeatable; default: ~/.config/"+appName+"/nodes/*.yml)")
	rootCmd.Flags().StringArrayVar(&flagRegistryDirs, "registry-dir", nil,
		"additional registry directory to scan for type files (repeatable)")

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		if isFlagInterceptError(err) {
			fmt.Fprintln(os.Stderr, "\nhint: flags after the command path are intercepted by "+appName+"; use -- to pass them through:")
			fmt.Fprintln(os.Stderr, "  "+appName+" [path] -- <flags>")
			fmt.Fprintln(os.Stderr, "  example: "+appName+" gene infra down -- -v")
		}
		os.Exit(1)
	}
}

// isFlagInterceptError reports whether the error is cobra intercepting a flag
// that was meant for the underlying command.
func isFlagInterceptError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unknown flag:") || strings.Contains(msg, "unknown shorthand flag:")
}

// load resolves config from flags and builds the runtime tree.
func load(flagFiles, flagRegistryDirs []string) (*dsl.Container, error) {
	configDir, err := resolveConfigDir()
	if err != nil {
		return nil, err
	}
	registryDirs := resolveRegistryDirs(configDir, flagRegistryDirs)
	nodeFiles, err := resolveNodeFiles(configDir, flagFiles)
	if err != nil {
		return nil, err
	}
	return loadSources(registryDirs, nodeFiles)
}

// resolvePath walks args greedily to find the target executable node.
//
// It returns either a *Runnable or a *Pipeline. Once a Runnable is reached,
// any remaining args are returned as extra args to be appended to the command.
// Use -- to pass flag-like extra args without Cobra intercepting them
// (e.g. devshell backend build -- --race).
//
// Pipelines are leaf nodes: reaching one during traversal stops the walk.
// Extra args are not supported for pipelines (steps have fixed argv).
func resolvePath(root *dsl.Container, args []string) (dsl.Node, []string, error) {
	var current dsl.Node = root
	var navigated []string

	for i, arg := range args {
		if r, ok := dsl.AsRunnable(current); ok {
			// Already at a Runnable: the remaining args go to the command.
			return r, args[i:], nil
		}
		if _, ok := dsl.AsPipeline(current); ok {
			// Already at a Pipeline: it is a leaf — remaining args are not forwarded.
			return current, nil, nil
		}
		c, _ := dsl.AsContainer(current)
		child, ok := c.Find(arg)
		if !ok {
			return nil, nil, notFoundError(arg, navigated, c)
		}
		navigated = append(navigated, arg)
		current = child
	}

	if r, ok := dsl.AsRunnable(current); ok {
		return r, nil, nil
	}
	if _, ok := dsl.AsPipeline(current); ok {
		return current, nil, nil
	}
	c, _ := dsl.AsContainer(current)
	return nil, nil, isContainerError(navigated, c)
}

// execute runs a Runnable with its configured cwd and env.
// Extra args are appended to the argv. No implicit shell is used.
func execute(r *dsl.Runnable, extraArgs []string) error {
	argv := append(append([]string(nil), r.Argv...), extraArgs...)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if r.Cwd != "" {
		cmd.Dir = r.Cwd
	}

	// Inherit the current environment and overlay with node-specific variables.
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	return cmd.Run()
}

// dynamicCompletion provides shell completion for devshell commands.
func dynamicCompletion(flagFiles, flagRegistryDirs, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	root, err := load(flagFiles, flagRegistryDirs)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	node, ok := navigate(root, args)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	c, ok := dsl.AsContainer(node)
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var suggestions []string
	for _, child := range c.Children {
		if strings.HasPrefix(child.Name(), toComplete) {
			suggestions = append(suggestions, child.Name())
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// navigate walks the tree following path segments and returns the deepest reachable node.
// Unlike resolvePath, it does not error on containers — it is used for completion.
func navigate(root *dsl.Container, path []string) (dsl.Node, bool) {
	var current dsl.Node = root
	for _, seg := range path {
		c, ok := dsl.AsContainer(current)
		if !ok {
			return nil, false
		}
		child, ok := c.Find(seg)
		if !ok {
			return nil, false
		}
		current = child
	}
	return current, true
}

// notFoundError reports which command was not found and lists valid alternatives.
func notFoundError(arg string, navigated []string, c *dsl.Container) error {
	location := "top level"
	if len(navigated) > 0 {
		location = fmt.Sprintf("'%s'", strings.Join(navigated, " "))
	}
	return fmt.Errorf("%q not found at %s\navailable: %s", arg, location, childList(c))
}

// isContainerError reports that a container was reached instead of a runnable,
// and lists its available subcommands.
func isContainerError(navigated []string, c *dsl.Container) error {
	name := strings.Join(navigated, " ")
	if name == "" {
		name = "top level"
	}
	return fmt.Errorf("%q is a container, not a runnable\navailable subcommands: %s", name, childList(c))
}

// childList returns a human-readable comma-separated list of a container's child names.
func childList(c *dsl.Container) string {
	names := make([]string, len(c.Children))
	for i, child := range c.Children {
		names[i] = child.Name()
	}
	return strings.Join(names, ", ")
}
