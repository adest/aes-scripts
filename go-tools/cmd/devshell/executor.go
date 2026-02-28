package main

import (
	"bufio"
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

// execute runs a Runnable with its configured cwd and env.
// Extra args are appended to the argv. No implicit shell is used.
// providedInputs contains any key=value inputs passed on the command line.
func execute(r *dsl.Runnable, extraArgs []string, providedInputs map[string]string) error {
	inputValues, err := collectInputs(r.Inputs, providedInputs)
	if err != nil {
		return fmt.Errorf("runnable %q: %w", r.NodeName, err)
	}

	argv := resolveInputRefsInSlice(append(append([]string(nil), r.Argv...), extraArgs...), inputValues)
	cwd := resolveInputRefs(r.Cwd, inputValues)
	env := resolveInputRefsInMap(r.Env, inputValues)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if cwd != "" {
		cmd.Dir = cwd
	}

	// Inherit the current environment and overlay with node-specific variables.
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	return cmd.Run()
}

// stepRefRunRe matches {{ steps.<id>.<stream> }} patterns at runtime.
// These are resolved by substituting the captured output of the named step.
var stepRefRunRe = regexp.MustCompile(`\{\{\s*steps\.([A-Za-z0-9_][A-Za-z0-9_-]*)\.(\w+)\s*\}\}`)

// inputRefRunRe matches {{ inputs.<name> }} patterns at runtime.
var inputRefRunRe = regexp.MustCompile(`\{\{\s*inputs\.([A-Za-z0-9_][A-Za-z0-9_-]*)\s*\}\}`)

// executePipeline runs all steps of a Pipeline in order.
//
// Captured outputs are stored in a map keyed by "<stepID>.stdout" or
// "<stepID>.stderr". Before each step runs, any {{ steps.X.stream }} patterns
// in its argv, env values, and cwd are replaced with the stored values.
// providedInputs contains any key=value inputs passed on the command line.
func executePipeline(p *dsl.Pipeline, providedInputs map[string]string) error {
	inputValues, err := collectInputs(p.Inputs, providedInputs)
	if err != nil {
		return fmt.Errorf("pipeline %q: %w", p.NodeName, err)
	}

	captures := make(map[string]string)

	for i, step := range p.Steps {
		if err := runStepWithPolicy(step, captures, inputValues); err != nil {
			// Always include the index; append the id when present.
			// e.g. "step [1]" or "step [1] (list)"
			label := fmt.Sprintf("[%d]", i)
			if step.ID != "" {
				label = fmt.Sprintf("[%d] (%s)", i, step.ID)
			}
			return fmt.Errorf("pipeline %q: step %s: %w", p.NodeName, label, err)
		}
	}
	return nil
}

// runStepWithPolicy runs a step once, respecting its on-fail policy.
//
//   - "" / "fail" (default): return the error immediately.
//   - "continue": run the step but swallow any error.
//   - "retry": retry up to OnFail.Attempts times, pausing OnFail.Delay between tries.
func runStepWithPolicy(step dsl.PipelineStep, captures map[string]string, inputValues map[string]string) error {
	of := step.OnFail

	switch of.Action {
	case "", "fail":
		return runStep(step, captures, inputValues)

	case "continue":
		runStep(step, captures, inputValues) // error intentionally ignored
		return nil

	case "retry":
		delay, _ := time.ParseDuration(of.Delay) // validated earlier; ignore parse error
		var lastErr error
		for attempt := 0; attempt < of.Attempts; attempt++ {
			if lastErr = runStep(step, captures, inputValues); lastErr == nil {
				return nil
			}
			if attempt < of.Attempts-1 && delay > 0 {
				time.Sleep(delay)
			}
		}
		return fmt.Errorf("failed after %d attempts: %w", of.Attempts, lastErr)

	default:
		// Should never happen: the validator rejects unknown actions.
		return runStep(step, captures, inputValues)
	}
}

// runStep executes a single pipeline step once and stores its captured output.
//
// Step-output references ({{ steps.X.stream }}) and input references
// ({{ inputs.name }}) in argv, env, and cwd are substituted before the
// command is built. On success, any captured output is written back into
// captures for later steps.
func runStep(step dsl.PipelineStep, captures map[string]string, inputValues map[string]string) error {
	// Resolve {{ steps.X.stream }} first, then {{ inputs.name }}.
	argv := resolveInputRefsInSlice(resolveRefsInSlice(step.Argv, captures), inputValues)
	cwd := resolveInputRefs(resolveRefs(step.Cwd, captures), inputValues)
	env := resolveInputRefsInMap(resolveRefsInMap(step.Env, captures), inputValues)

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

// ---------------------------------------------------------------------------
// Input collection and resolution
// ---------------------------------------------------------------------------

// collectInputs resolves the final input values for a node.
//
// For each declared input:
//   - If provided on the command line → use it.
//   - If omitted and has a default → use the default.
//   - If required and not provided → prompt the user interactively.
func collectInputs(decls map[string]*string, provided map[string]string) (map[string]string, error) {
	if len(decls) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(decls))
	for name, defaultVal := range decls {
		if v, ok := provided[name]; ok {
			result[name] = v
		} else if defaultVal != nil {
			result[name] = *defaultVal
		} else {
			v, err := promptInput(name)
			if err != nil {
				return nil, fmt.Errorf("%w: %s", dsl.ErrMissingInput, name)
			}
			result[name] = v
		}
	}
	return result, nil
}

// promptInput interactively prompts the user for the value of a required input.
// Returns an error if the value is empty or stdin is closed.
func promptInput(name string) (string, error) {
	fmt.Fprintf(os.Stderr, "input %q: ", name)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", fmt.Errorf("empty value provided for required input %q", name)
	}
	return line, nil
}

// resolveInputRefs replaces every {{ inputs.<name> }} occurrence in s with
// the corresponding value from inputValues. Unknown references are left as-is.
func resolveInputRefs(s string, inputValues map[string]string) string {
	if len(inputValues) == 0 {
		return s
	}
	return inputRefRunRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := inputRefRunRe.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		if val, ok := inputValues[sub[1]]; ok {
			return val
		}
		return match
	})
}

// resolveInputRefsInSlice applies resolveInputRefs to every element of a slice.
func resolveInputRefsInSlice(ss []string, inputValues map[string]string) []string {
	if len(inputValues) == 0 {
		return ss
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = resolveInputRefs(s, inputValues)
	}
	return out
}

// resolveInputRefsInMap applies resolveInputRefs to every value in a map.
func resolveInputRefsInMap(m map[string]string, inputValues map[string]string) map[string]string {
	if len(m) == 0 || len(inputValues) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = resolveInputRefs(v, inputValues)
	}
	return out
}

// ---------------------------------------------------------------------------
// Step-ref resolution helpers
// ---------------------------------------------------------------------------

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
