package main

import (
	"fmt"
	"sort"
	"strings"

	"go-tools/cmd/devshell/dsl"
)

// dryRunRunnable prints the command that would be executed for a Runnable.
func dryRunRunnable(r *dsl.Runnable, extraArgs []string) {
	argv := append(append([]string(nil), r.Argv...), extraArgs...)
	fmt.Printf("[dry-run] runnable %q\n", r.NodeName)
	fmt.Printf("  command: %s\n", strings.Join(argv, " "))
	if r.Cwd != "" {
		fmt.Printf("  cwd:     %s\n", r.Cwd)
	}
	if len(r.Env) > 0 {
		fmt.Println("  env:")
		for _, k := range sortedKeys(r.Env) {
			fmt.Printf("    %s=%s\n", k, r.Env[k])
		}
	}
}

// dryRunPipeline prints each step that would be executed for a Pipeline.
func dryRunPipeline(p *dsl.Pipeline) {
	fmt.Printf("[dry-run] pipeline %q\n", p.NodeName)
	for i, step := range p.Steps {
		fmt.Println()
		if step.ID != "" {
			fmt.Printf("  step [%d] (%s)\n", i, step.ID)
		} else {
			fmt.Printf("  step [%d]\n", i)
		}
		fmt.Printf("    command: %s\n", strings.Join(step.Argv, " "))
		if step.Cwd != "" {
			fmt.Printf("    cwd:     %s\n", step.Cwd)
		}
		if len(step.Env) > 0 {
			fmt.Println("    env:")
			for _, k := range sortedKeys(step.Env) {
				fmt.Printf("      %s=%s\n", k, step.Env[k])
			}
		}
		if step.Stdin != nil {
			fmt.Printf("    stdin:   ← %s.%s\n", step.Stdin.StepID, streamKey(step.Stdin.Stream))
		}
		if step.Capture != dsl.CaptureNone {
			cap := dryCapture(step.Capture)
			if step.ID != "" && step.Tee {
				fmt.Printf("    capture: %s → %q (tee)\n", cap, step.ID)
			} else if step.ID != "" {
				fmt.Printf("    capture: %s → %q\n", cap, step.ID)
			} else {
				fmt.Printf("    capture: %s\n", cap)
			}
		}
		switch step.OnFail.Action {
		case "continue":
			fmt.Println("    on_fail: continue")
		case "retry":
			if step.OnFail.Delay != "" && step.OnFail.Delay != "0s" {
				fmt.Printf("    on_fail: retry (%d attempts, %s delay)\n", step.OnFail.Attempts, step.OnFail.Delay)
			} else {
				fmt.Printf("    on_fail: retry (%d attempts)\n", step.OnFail.Attempts)
			}
		}
	}
}

func dryCapture(m dsl.CaptureMode) string {
	switch m {
	case dsl.CaptureStdout:
		return "stdout"
	case dsl.CaptureStderr:
		return "stderr"
	case dsl.CaptureBoth:
		return "stdout+stderr"
	default:
		return "none"
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
