package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model struct for Bubbletea
type model struct {
	context     string
	kubeconfig  string
	input       textinput.Model
	output      string
	errorDetail string
	list        list.Model
	state       string // "main", "select", "error"
	files       []string
}

// listItem for Bubbles list
type listItem string

func (i listItem) FilterValue() string { return string(i) }
func (i listItem) Title() string       { return filepath.Base(string(i)) }
func (i listItem) Description() string { return string(i) }

// listItemDelegate for Bubbles list
type listItemDelegate struct{}

func (d listItemDelegate) Height() int               { return 1 }
func (d listItemDelegate) Spacing() int              { return 0 }
func (d listItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d listItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if li, ok := item.(listItem); ok {
		fmt.Fprint(w, li.Title())
	} else {
		fmt.Fprint(w, fmt.Sprintf("%v", item))
	}
}

// View renders the TUI
func (m model) View() string {
	// Lipgloss styles
	border := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2).Margin(1, 0)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36"))
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
	iconStyle := lipgloss.NewStyle().Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	resultStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	actionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))

	ctxIcon := "⎈"
	fileIcon := "📄"
	cmdIcon := "💻"
	resultIcon := "✅"
	errorIcon := "❌"
	selectIcon := "🗂️"

	// Context section
	contextSection := titleStyle.Render(ctxIcon + " KUBECTL CONTEXT") + "\n" +
		iconStyle.Render(ctxIcon) + " " + labelStyle.Render("Current context:") + " " + valueStyle.Render(m.context) + "\n" +
		iconStyle.Render(fileIcon) + " " + labelStyle.Render("Selected kubeconfig:") + " " + valueStyle.Render(m.kubeconfig)

	// Command result section
	resultSection := ""
	lastCmd := ""
	if m.state == "main" && m.output != "" {
		lastCmd = iconStyle.Render(cmdIcon) + " " + labelStyle.Render("Last command:") + " " + valueStyle.Render(m.input.Value()) + "\n"
		resultSection += labelStyle.Render("Output:") + "\n"
		for _, line := range strings.Split(m.output, "\n") {
			resultSection += "  "
			if strings.Contains(line, "Error:") {
				resultSection += errorStyle.Render(errorIcon + " " + line) + "\n"
			} else {
				resultSection += resultStyle.Render(resultIcon + " " + line) + "\n"
			}
		}
		if m.errorDetail != "" {
			resultSection += actionStyle.Render("(Press 'e' to view error details)") + "\n"
		}
	}

	// Error details section
	errorSection := ""
	if m.state == "error" {
		errorSection += titleStyle.Render(errorIcon + " ERROR DETAILS") + "\n"
		for _, line := range strings.Split(m.errorDetail, "\n") {
			errorSection += "  " + errorStyle.Render(line) + "\n"
		}
		errorSection += actionStyle.Render("(Press 'e' to return)") + "\n"
	}

	// Kubeconfig selection section
	selectSection := ""
	if m.state == "select" {
		selectSection += titleStyle.Render(selectIcon + " KUBECONFIG SELECTION") + "\n"
		for _, line := range strings.Split(m.list.View(), "\n") {
			selectSection += "  " + valueStyle.Render(line) + "\n"
		}
		selectSection += actionStyle.Render("Actions: [Enter] Select | [q] Cancel") + "\n"
	}

	// Command input section (always at the end)
	inputSection := titleStyle.Render(cmdIcon + " COMMAND INPUT") + "\n" +
		iconStyle.Render(cmdIcon) + " " + labelStyle.Render("Type a kubectl command (e.g. get pods):") + "\n" +
		"  " + m.input.View() + "\n" +
		actionStyle.Render("Actions: [Enter] Run | [Tab] Change kubeconfig | [q] Quit")

	// Compose all sections
	content := contextSection + "\n\n" + lastCmd + resultSection + errorSection + selectSection + "\n" + inputSection
	return border.Render(content)
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.state = "select"
			return m, nil
		case "e":
			if m.state == "main" && m.errorDetail != "" {
				m.state = "error"
				return m, nil
			} else if m.state == "error" {
				m.state = "main"
				return m, nil
			}
		case "enter":
			if m.state == "main" {
				args := strings.Fields(m.input.Value())
				cmd := exec.Command("kubectl", args...)
				env := os.Environ()
				if m.kubeconfig != "" {
					env = append(env, "KUBECONFIG="+m.kubeconfig)
				}
				cmd.Env = env
				out, err := cmd.CombinedOutput()
				if err != nil {
					m.output = "Error: " + err.Error()
					m.errorDetail = string(out)
				} else {
					m.output = string(out)
					m.errorDetail = ""
				}
				m.context = getKubectlContext(m.kubeconfig)
				m.input.SetValue("")
				return m, nil
			} else if m.state == "select" {
				idx := m.list.Index()
				if idx >= 0 && idx < len(m.files) {
					m.kubeconfig = m.files[idx]
					m.context = getKubectlContext(m.kubeconfig)
				}
				m.state = "main"
				return m, nil
			}
		}
	}
	if m.state == "main" {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	} else if m.state == "select" {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}


// Add getKubectlContext function
func getKubectlContext(kubeconfig string) string {
	args := []string{"config", "current-context"}
	cmd := exec.Command("kubectl", args...)
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	}
	out, err := cmd.Output()
	if err != nil {
		return "(error: " + err.Error() + ")"
	}
	return strings.TrimSpace(string(out))
}

// Add initialModel function
func initialModel() model {
	input := textinput.New()
	input.Placeholder = "kubectl command (e.g. get pods)"
	input.Focus()
	// Find kubeconfig files in Downloads
	home, _ := os.UserHomeDir()
	pattern := home + "/Downloads/config*.yml"
	files, _ := filepath.Glob(pattern)
	if len(files) == 0 {
		files = []string{""}
	}
	items := make([]list.Item, len(files))
	for i, f := range files {
		items[i] = listItem(f)
	}
	l := list.New(items, listItemDelegate{}, 40, 10)
	l.Title = "Select kubeconfig file"
	// Default to first kubeconfig if available
	kubeconfig := ""
	if len(files) > 0 && files[0] != "" {
		kubeconfig = files[0]
	}
	return model{
		context:    getKubectlContext(kubeconfig),
		kubeconfig: kubeconfig,
		input:      input,
		output:     "",
		errorDetail: "",
		list:       l,
		state:      "main",
		files:      files,
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
