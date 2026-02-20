package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/derekurban2001/authmux/internal/adapters"
	"github.com/derekurban2001/authmux/internal/app"
	"github.com/derekurban2001/authmux/internal/shim"
	"github.com/derekurban2001/authmux/internal/store"
)

type mode int

const (
	modeNormal mode = iota
	modeAddTool
	modeAddName
	modeConfirmDelete
)

type loadMsg struct {
	state *store.State
	rows  []app.StatusRow
	err   error
}

type actionDoneMsg struct {
	err error
}

type model struct {
	manager *app.Manager
	state   *store.State
	rows    []app.StatusRow
	cursor  int
	width   int
	height  int
	mode    mode
	message string

	toolIndex int
	nameInput textinput.Model
}

func Run(manager *app.Manager) error {
	input := textinput.New()
	input.Placeholder = "profile name"
	input.CharLimit = 64
	input.Prompt = "> "
	m := model{
		manager:   manager,
		state:     &store.State{Defaults: map[store.Tool]string{}},
		rows:      []app.StatusRow{},
		mode:      modeNormal,
		message:   "Welcome to AuthMux. Press 'a' to add your first profile.",
		nameInput: input,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return m.loadCmd()
}

func (m model) loadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		st, err := m.manager.Load()
		if err != nil {
			return loadMsg{err: err}
		}
		rows, err := m.manager.StatusRows(ctx, nil)
		if err != nil {
			return loadMsg{state: st, err: err}
		}
		return loadMsg{state: st, rows: rows}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case loadMsg:
		if msg.err != nil {
			m.message = "Error: " + msg.err.Error()
			return m, nil
		}
		m.state = msg.state
		m.rows = msg.rows
		if len(m.state.Profiles) == 0 {
			m.cursor = 0
		} else if m.cursor >= len(m.state.Profiles) {
			m.cursor = len(m.state.Profiles) - 1
		}
		return m, nil
	case actionDoneMsg:
		if msg.err != nil {
			m.message = "Action failed: " + msg.err.Error()
		} else {
			m.message = "Done."
		}
		return m, m.loadCmd()
	case tea.KeyMsg:
		switch m.mode {
		case modeAddTool:
			return m.updateAddTool(msg)
		case modeAddName:
			return m.updateAddName(msg)
		case modeConfirmDelete:
			return m.updateConfirmDelete(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.state.Profiles)-1 {
			m.cursor++
		}
	case "r":
		m.message = "Refreshing statuses..."
		return m, m.loadCmd()
	case "a":
		m.mode = modeAddTool
		m.toolIndex = 0
		m.message = "Choose tool for new profile"
	case "u":
		p, ok := m.selectedProfile()
		if !ok {
			m.message = "No profile selected"
			return m, nil
		}
		if err := m.manager.SetDefault(p.Tool, p.Name); err != nil {
			m.message = "Failed to set default: " + err.Error()
			return m, nil
		}
		m.message = fmt.Sprintf("Default set: %s/%s", p.Tool, p.Name)
		return m, m.loadCmd()
	case "d":
		if len(m.state.Profiles) == 0 {
			m.message = "No profile selected"
			return m, nil
		}
		m.mode = modeConfirmDelete
	case "s":
		return m.installShims()
	case "enter":
		p, ok := m.selectedProfile()
		if !ok {
			m.message = "No profile selected"
			return m, nil
		}
		adapter, err := adapters.Get(p.Tool)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		cmd := adapter.RunCommand(p.Dir, []string{})
		m.message = fmt.Sprintf("Launching %s/%s...", p.Tool, p.Name)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return actionDoneMsg{err: err} })
	case "l":
		p, ok := m.selectedProfile()
		if !ok {
			m.message = "No profile selected"
			return m, nil
		}
		adapter, err := adapters.Get(p.Tool)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		cmd := adapter.LoginCommand(p.Dir)
		m.message = fmt.Sprintf("Login flow started for %s/%s", p.Tool, p.Name)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return actionDoneMsg{err: err} })
	case "o":
		p, ok := m.selectedProfile()
		if !ok {
			m.message = "No profile selected"
			return m, nil
		}
		adapter, err := adapters.Get(p.Tool)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		cmd := adapter.LogoutCommand(p.Dir)
		m.message = fmt.Sprintf("Logging out %s/%s", p.Tool, p.Name)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return actionDoneMsg{err: err} })
	}
	return m, nil
}

func (m model) updateAddTool(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.mode = modeNormal
		m.message = "Add cancelled"
		return m, nil
	case "up", "k":
		if m.toolIndex > 0 {
			m.toolIndex--
		}
	case "down", "j":
		if m.toolIndex < len(store.SupportedTools)-1 {
			m.toolIndex++
		}
	case "enter":
		m.mode = modeAddName
		m.nameInput.SetValue("")
		m.nameInput.Focus()
		m.message = fmt.Sprintf("Enter profile name for %s", store.SupportedTools[m.toolIndex])
		return m, textinput.Blink
	}
	return m, nil
}

func (m model) updateAddName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.nameInput.Blur()
		m.message = "Add cancelled"
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.message = "Profile name cannot be empty"
			return m, nil
		}
		tool := store.SupportedTools[m.toolIndex]
		profile, _, err := m.manager.EnsureProfile(tool, name)
		if err != nil {
			m.message = "Failed: " + err.Error()
			return m, nil
		}
		adapter, err := adapters.Get(profile.Tool)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.mode = modeNormal
		m.nameInput.Blur()
		m.message = fmt.Sprintf("Launching login for %s/%s", profile.Tool, profile.Name)
		cmd := adapter.LoginCommand(profile.Dir)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return actionDoneMsg{err: err} })
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		p, ok := m.selectedProfile()
		if !ok {
			m.mode = modeNormal
			return m, nil
		}
		err := m.manager.RemoveProfile(p.Tool, p.Name, false)
		m.mode = modeNormal
		if err != nil {
			m.message = "Delete failed: " + err.Error()
			return m, nil
		}
		m.message = fmt.Sprintf("Removed %s/%s", p.Tool, p.Name)
		return m, m.loadCmd()
	case "n", "esc":
		m.mode = modeNormal
		m.message = "Delete cancelled"
	}
	return m, nil
}

func (m model) installShims() (tea.Model, tea.Cmd) {
	if len(m.state.Profiles) == 0 {
		m.message = "No profiles to shim"
		return m, nil
	}
	dir, err := shim.DefaultShimDir()
	if err != nil {
		m.message = "Failed to detect shim dir: " + err.Error()
		return m, nil
	}
	authmuxBin, err := exec.LookPath("authmux")
	if err != nil {
		authmuxBin = "authmux"
	}
	count := 0
	for _, p := range m.state.Profiles {
		if _, err := shim.Install(dir, p, authmuxBin); err == nil {
			count++
		}
	}
	m.message = fmt.Sprintf("Installed %d shim(s) in %s", count, dir)
	return m, nil
}

func (m model) selectedProfile() (store.Profile, bool) {
	if len(m.state.Profiles) == 0 || m.cursor < 0 || m.cursor >= len(m.state.Profiles) {
		return store.Profile{}, false
	}
	return m.state.Profiles[m.cursor], true
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading AuthMux..."
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("AuthMux") + "  " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("profile-based auth launcher for Claude + Codex")

	left := m.renderProfilesPane()
	right := m.renderDetailPane()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(m.footerHelp())
	msg := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(m.message)

	view := lipgloss.JoinVertical(lipgloss.Left, header, body, msg, footer)
	return lipgloss.NewStyle().Padding(1, 2).Render(view)
}

func (m model) renderProfilesPane() string {
	width := max(36, m.width/3)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Render("Profiles")
	if len(m.state.Profiles) == 0 {
		card := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2).
			Width(width - 2).Render("No profiles yet.\n\nPress 'a' to create one.")
		return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, title, card))
	}
	rowsByKey := map[string]app.StatusRow{}
	for _, r := range m.rows {
		rowsByKey[string(r.Profile.Tool)+"/"+r.Profile.Name] = r
	}
	lines := []string{}
	for i, p := range m.state.Profiles {
		key := string(p.Tool) + "/" + p.Name
		row, ok := rowsByKey[key]
		statusIcon := "…"
		if ok {
			if row.Error != "" {
				statusIcon = "⚠"
			} else if row.Status.LoggedIn {
				statusIcon = "●"
			} else {
				statusIcon = "○"
			}
		}
		defaultMarker := " "
		if m.state.Defaults[p.Tool] == p.Name {
			defaultMarker = "*"
		}
		line := fmt.Sprintf("%s %s %s/%s", defaultMarker, statusIcon, p.Tool, p.Name)
		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Render(" " + line + " ")
		}
		lines = append(lines, line)
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 1).Width(width - 2)
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, title, box.Render(strings.Join(lines, "\n"))))
}

func (m model) renderDetailPane() string {
	width := m.width - max(36, m.width/3) - 8
	if width < 40 {
		width = 40
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")).Render("Details")
	content := "Select a profile"
	if p, ok := m.selectedProfile(); ok {
		status := "unknown"
		errText := ""
		for _, r := range m.rows {
			if r.Profile.Tool == p.Tool && r.Profile.Name == p.Name {
				if r.Error != "" {
					status = "error"
					errText = r.Error
				} else if r.Status.LoggedIn {
					status = "logged in"
				} else {
					status = "logged out"
				}
				break
			}
		}
		content = fmt.Sprintf("Tool: %s\nProfile: %s\nDefault: %v\nDir: %s\nStatus: %s", p.Tool, p.Name, m.state.Defaults[p.Tool] == p.Name, p.Dir, status)
		if errText != "" {
			content += "\nError: " + errText
		}
	}

	modal := ""
	switch m.mode {
	case modeAddTool:
		toolLines := []string{"Choose tool:"}
		for i, t := range store.SupportedTools {
			prefix := "  "
			if i == m.toolIndex {
				prefix = "> "
			}
			toolLines = append(toolLines, prefix+string(t))
		}
		modal = strings.Join(toolLines, "\n")
	case modeAddName:
		modal = "New profile name:\n" + m.nameInput.View()
	case modeConfirmDelete:
		if p, ok := m.selectedProfile(); ok {
			modal = fmt.Sprintf("Delete %s/%s from registry? (y/n)", p.Tool, p.Name)
		}
	}

	actions := "Actions:\n[Enter] Launch\n[a] Add profile\n[l] Login\n[o] Logout\n[u] Set default\n[d] Remove\n[s] Install shims\n[r] Refresh\n[q] Quit"
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2).Width(width)
	body := content + "\n\n" + actions
	if modal != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(1, 2).Render(modal)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, box.Render(body))
}

func (m model) footerHelp() string {
	return "* = default profile · ● logged in · ○ logged out · ⚠ status check failed"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
