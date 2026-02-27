package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type appState int

const (
	stateList appState = iota
	stateConfirm
	stateResult
)

var (
	styleBase = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Padding(0, 1)

	styleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	styleOverlay = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 3).
			MarginLeft(2)

	styleOverlayTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214"))

	styleKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99"))

	styleOK = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Padding(0, 1)

	styleErr = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Padding(0, 1)
)

type model struct {
	table       table.Model
	connections []ConnectionInfo
	proto       string
	state       appState
	resultMsg   string
	resultErr   error
}

func newModel(connections []ConnectionInfo, proto string) model {
	columns := []table.Column{
		{Title: "PROTO", Width: 6},
		{Title: "ADRESSE", Width: 22},
		{Title: "PID", Width: 8},
		{Title: "PROCESSUS", Width: 22},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(toRows(connections)),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("99"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return model{
		table:       t,
		connections: connections,
		proto:       proto,
		state:       stateList,
	}
}

func toRows(conns []ConnectionInfo) []table.Row {
	rows := make([]table.Row, len(conns))
	for i, c := range conns {
		rows[i] = table.Row{c.Proto, c.LocalAddr, fmt.Sprintf("%d", c.Pid), c.ProcessName}
	}
	return rows
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateList:
		return m.updateList(msg)
	case stateConfirm:
		return m.updateConfirm(msg)
	case stateResult:
		return m.updateResult(msg)
	}
	return m, nil
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+k":
			if len(m.connections) > 0 {
				m.state = stateConfirm
			}
			return m, nil
		case "r":
			conns, err := ScanConnections(m.proto)
			if err == nil {
				m.connections = conns
				m.table.SetRows(toRows(conns))
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.connections) {
				pid := m.connections[idx].Pid
				err := killProcess(pid)
				m.resultErr = err
				if err == nil {
					m.resultMsg = fmt.Sprintf("Process PID %d tué avec succès.", pid)
				} else {
					m.resultMsg = fmt.Sprintf("Échec kill PID %d : %v", pid, err)
				}
			}
			m.state = stateResult
			return m, nil
		case "n", "esc", "q":
			m.state = stateList
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "enter", "r":
			conns, err := ScanConnections(m.proto)
			if err == nil {
				m.connections = conns
				m.table.SetRows(toRows(conns))
			}
			m.state = stateList
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	protoLabel := strings.ToUpper(m.proto)
	title := styleTitle.Render("TCPO  [" + protoLabel + "]  — ports en écoute")
	tableView := styleBase.Render(m.table.View())

	switch m.state {
	case stateConfirm:
		var target string
		if idx := m.table.Cursor(); idx >= 0 && idx < len(m.connections) {
			c := m.connections[idx]
			target = fmt.Sprintf("%s  (PID %d)", c.ProcessName, c.Pid)
		}
		overlay := styleOverlay.Render(
			styleOverlayTitle.Render("Kill "+target+" ?") + "\n\n" +
				styleKey.Render("y") + " confirmer    " +
				styleKey.Render("n") + " annuler",
		)
		return title + "\n" + tableView + "\n" + overlay

	case stateResult:
		var msg string
		if m.resultErr != nil {
			msg = styleErr.Render(m.resultMsg)
		} else {
			msg = styleOK.Render(m.resultMsg)
		}
		help := styleHelp.Render("Entrée / r  continuer    q  quitter")
		return title + "\n" + tableView + "\n" + msg + "\n" + help

	default:
		var help string
		if len(m.connections) == 0 {
			help = styleHelp.Render("Aucun port en écoute.  r  rafraîchir    q  quitter")
		} else {
			help = styleHelp.Render("↑/↓  naviguer    ctrl+k  kill    r  rafraîchir    q  quitter")
		}
		return title + "\n" + tableView + "\n" + help
	}
}
