package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"go-tools/cmd/devshell/dsl"
)

// stepRefRunRe matches {{ steps.<id>.<stream> }} patterns at runtime.
// These are resolved by substituting the captured output of the named step.
var stepRefRunRe = regexp.MustCompile(`\{\{\s*steps\.([A-Za-z0-9_][A-Za-z0-9_-]*)\.(\w+)\s*\}\}`)

// executePipeline runs all steps of a Pipeline in order.
//
// Captured outputs are stored in a map keyed by "<stepID>.stdout" or
// "<stepID>.stderr". Before each step runs, any {{ steps.X.stream }} patterns
// in its argv, env values, and cwd are replaced with the stored values.
func executePipeline(p *dsl.Pipeline) error {
	captures := make(map[string]string)

	for i, step := range p.Steps {
		if err := runStepWithPolicy(step, captures); err != nil {
			// Use the step ID in the error message when available.
			label := fmt.Sprintf("[%d]", i)
			if step.ID != "" {
				label = step.ID
			}
			return fmt.Errorf("step %s: %w", label, err)
		}
	}
	return nil
}

// runStepWithPolicy runs a step once, respecting its on-fail policy.
//
//   - "" / "fail" (default): return the error immediately.
//   - "continue": run the step but swallow any error.
//   - "retry": retry up to OnFail.Attempts times, pausing OnFail.Delay between tries.
func runStepWithPolicy(step dsl.PipelineStep, captures map[string]string) error {
	of := step.OnFail

	switch of.Action {
	case "", "fail":
		return runStep(step, captures)

	case "continue":
		runStep(step, captures) // error intentionally ignored
		return nil

	case "retry":
		delay, _ := time.ParseDuration(of.Delay) // validated earlier; ignore parse error
		var lastErr error
		for attempt := 0; attempt < of.Attempts; attempt++ {
			if lastErr = runStep(step, captures); lastErr == nil {
				return nil
			}
			if attempt < of.Attempts-1 && delay > 0 {
				time.Sleep(delay)
			}
		}
		return fmt.Errorf("failed after %d attempts: %w", of.Attempts, lastErr)

	default:
		// Should never happen: the validator rejects unknown actions.
		return runStep(step, captures)
	}
}

// runStep executes a single pipeline step once and stores its captured output.
//
// Step-output references ({{ steps.X.stream }}) in argv, env, and cwd are
// substituted with values from captures before the command is built.
// On success, any captured output is written back into captures for later steps.
func runStep(step dsl.PipelineStep, captures map[string]string) error {
	// Resolve {{ steps.X.stream }} in argv, cwd, and env values.
	argv := resolveRefsInSlice(step.Argv, captures)
	cwd := resolveRefs(step.Cwd, captures)
	env := resolveRefsInMap(step.Env, captures)

	cmd := exec.Command(argv[0], argv[1:]...)

	// Working directory: empty means inherit from the parent process.
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Environment: start from the process env, then overlay step-specific vars.
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Stdout: capture to an in-memory buffer.
	// tee: true → also stream to the terminal so the user can see it.
	// No capture → pass through directly.
	var stdoutBuf bytes.Buffer
	if step.Capture == dsl.CaptureStdout || step.Capture == dsl.CaptureBoth {
		if step.Tee {
			cmd.Stdout = io.MultiWriter(&stdoutBuf, os.Stdout)
		} else {
			cmd.Stdout = &stdoutBuf
		}
	} else {
		cmd.Stdout = os.Stdout
	}

	// Stderr: same logic as stdout.
	var stderrBuf bytes.Buffer
	if step.Capture == dsl.CaptureStderr || step.Capture == dsl.CaptureBoth {
		if step.Tee {
			cmd.Stderr = io.MultiWriter(&stderrBuf, os.Stderr)
		} else {
			cmd.Stderr = &stderrBuf
		}
	} else {
		cmd.Stderr = os.Stderr
	}

	// Stdin: feed the captured output of a prior step, or inherit terminal stdin.
	if step.Stdin != nil {
		key := step.Stdin.StepID + "." + streamKey(step.Stdin.Stream)
		cmd.Stdin = strings.NewReader(captures[key])
	} else {
		cmd.Stdin = os.Stdin
	}

	if err := cmd.Run(); err != nil {
		return err
	}

	// Store captured output for later steps.
	// Trailing newlines are stripped so injected values behave like $() in shell.
	if step.ID != "" {
		if step.Capture == dsl.CaptureStdout || step.Capture == dsl.CaptureBoth {
			captures[step.ID+".stdout"] = strings.TrimRight(stdoutBuf.String(), "\r\n")
		}
		if step.Capture == dsl.CaptureStderr || step.Capture == dsl.CaptureBoth {
			captures[step.ID+".stderr"] = strings.TrimRight(stderrBuf.String(), "\r\n")
		}
	}

	return nil
}

// resolveRefs replaces every {{ steps.<id>.<stream> }} occurrence in s with
// the corresponding value from captures. Unknown references are left as-is.
func resolveRefs(s string, captures map[string]string) string {
	return stepRefRunRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := stepRefRunRe.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		key := sub[1] + "." + sub[2] // e.g. "list.stdout"
		if val, ok := captures[key]; ok {
			return val
		}
		return match
	})
}

// resolveRefsInSlice applies resolveRefs to every element of a string slice.
func resolveRefsInSlice(ss []string, captures map[string]string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = resolveRefs(s, captures)
	}
	return out
}

// resolveRefsInMap applies resolveRefs to every value in a map[string]string.
func resolveRefsInMap(m map[string]string, captures map[string]string) map[string]string {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = resolveRefs(v, captures)
	}
	return out
}

// streamKey converts a CaptureMode to the string suffix used in captures keys.
func streamKey(m dsl.CaptureMode) string {
	if m == dsl.CaptureStderr {
		return "stderr"
	}
	return "stdout"
}
