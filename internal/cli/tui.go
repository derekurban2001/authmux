package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/derekurban/profilex-cli/internal/store"
)

type tuiAction int

const (
	tuiActionAddProfile tuiAction = iota
	tuiActionSetDefault
	tuiActionSnapshot
	tuiActionApply
	tuiActionEnableSync
	tuiActionDisableSync
	tuiActionRefresh
	tuiActionQuit
)

type tuiMenuItem struct {
	Title  string
	Hint   string
	Action tuiAction
}

type tuiDataMsg struct {
	State   *store.State
	Presets []store.SettingsPreset
	Sync    []store.SettingsSync
	Native  map[store.Tool]tuiNativeInfo
}

type tuiErrorMsg struct {
	Err error
}

type tuiDoneMsg struct {
	Message string
}

type tuiNativeInfo struct {
	ConfigDir  string
	SessionDir string
}

type tuiForm struct {
	Action tuiAction
	Title  string
	Hint   string
	Fields []string
	Inputs []textinput.Model
	Focus  int
}

type tuiModel struct {
	rootDir string

	menu   []tuiMenuItem
	cursor int

	form *tuiForm
	busy bool

	state   *store.State
	presets []store.SettingsPreset
	sync    []store.SettingsSync
	native  map[store.Tool]tuiNativeInfo

	width  int
	height int

	info string
	err  string

	styles tuiStyles
}

type tuiStyles struct {
	header     lipgloss.Style
	panel      lipgloss.Style
	menuItem   lipgloss.Style
	menuActive lipgloss.Style
	label      lipgloss.Style
	error      lipgloss.Style
	info       lipgloss.Style
}

func cmdTUI(rootDir string, args []string) error {
	if hasHelp(args) {
		fmt.Printf("Usage: profilex tui\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %s", strings.Join(args, " "))
	}
	m := newTUIModel(rootDir)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newTUIModel(rootDir string) tuiModel {
	return tuiModel{
		rootDir: rootDir,
		menu: []tuiMenuItem{
			{Title: "Add Profile", Hint: "Create profile and optional shared sessions link", Action: tuiActionAddProfile},
			{Title: "Set Default", Hint: "Set default profile for a tool", Action: tuiActionSetDefault},
			{Title: "Snapshot Settings", Hint: "Capture tool-native settings into a named preset", Action: tuiActionSnapshot},
			{Title: "Apply Preset", Hint: "Apply named preset to a profile", Action: tuiActionApply},
			{Title: "Enable Sync", Hint: "Keep a profile synced to a preset", Action: tuiActionEnableSync},
			{Title: "Disable Sync", Hint: "Stop preset sync for a profile", Action: tuiActionDisableSync},
			{Title: "Refresh", Hint: "Reload state from disk", Action: tuiActionRefresh},
			{Title: "Quit", Hint: "Exit TUI", Action: tuiActionQuit},
		},
		styles: tuiStyles{
			header: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F8FAFC")).
				Background(lipgloss.Color("#1D4ED8")).
				Padding(0, 1).
				Bold(true),
			panel: lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#94A3B8")).
				Padding(1, 2),
			menuItem: lipgloss.NewStyle().
				Padding(0, 1),
			menuActive: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0F172A")).
				Background(lipgloss.Color("#A7F3D0")).
				Padding(0, 1).
				Bold(true),
			label: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#334155")).
				Bold(true),
			error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B91C1C")).
				Bold(true),
			info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#065F46")).
				Bold(true),
		},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tuiRefreshCmd(m.rootDir)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tuiDataMsg:
		m.state = msg.State
		m.presets = msg.Presets
		m.sync = msg.Sync
		m.native = msg.Native
		m.busy = false
		return m, nil
	case tuiDoneMsg:
		m.info = msg.Message
		m.err = ""
		m.form = nil
		m.busy = false
		return m, tuiRefreshCmd(m.rootDir)
	case tuiErrorMsg:
		m.err = msg.Err.Error()
		m.info = ""
		m.busy = false
		return m, nil
	}

	if m.busy {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}

	if m.form != nil {
		return m.updateForm(msg)
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.menu)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		item := m.menu[m.cursor]
		switch item.Action {
		case tuiActionQuit:
			return m, tea.Quit
		case tuiActionRefresh:
			m.info = "Refreshing..."
			m.err = ""
			m.busy = true
			return m, tuiRefreshCmd(m.rootDir)
		default:
			m.form = newTUIForm(item.Action)
			m.info = ""
			m.err = ""
			return m, nil
		}
	}

	return m, nil
}

func (m tuiModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			m.form = nil
			m.err = ""
			return m, nil
		case "tab", "down":
			m.form.Focus = (m.form.Focus + 1) % len(m.form.Inputs)
			return m.focusFormInput(), nil
		case "shift+tab", "up":
			m.form.Focus--
			if m.form.Focus < 0 {
				m.form.Focus = len(m.form.Inputs) - 1
			}
			return m.focusFormInput(), nil
		case "enter":
			if m.form.Focus == len(m.form.Inputs)-1 {
				values := map[string]string{}
				for i, field := range m.form.Fields {
					values[field] = strings.TrimSpace(m.form.Inputs[i].Value())
				}
				m.busy = true
				m.info = "Working..."
				m.err = ""
				return m, tuiRunActionCmd(m.rootDir, m.form.Action, values)
			}
			m.form.Focus++
			return m.focusFormInput(), nil
		}
	}

	cmds := make([]tea.Cmd, len(m.form.Inputs))
	for i := range m.form.Inputs {
		if i == m.form.Focus {
			m.form.Inputs[i].Focus()
		} else {
			m.form.Inputs[i].Blur()
		}
		var cmd tea.Cmd
		m.form.Inputs[i], cmd = m.form.Inputs[i].Update(msg)
		cmds[i] = cmd
	}
	return m, tea.Batch(cmds...)
}

func (m tuiModel) focusFormInput() tuiModel {
	for i := range m.form.Inputs {
		if i == m.form.Focus {
			m.form.Inputs[i].Focus()
		} else {
			m.form.Inputs[i].Blur()
		}
	}
	return m
}

func (m tuiModel) View() string {
	header := m.styles.header.Render("ProfileX TUI  |  Settings Snapshot + Sync  |  q:quit")

	left := m.renderProfilesPanel()
	rightTop := m.renderPresetsPanel()
	rightBottom := m.renderSyncPanel()

	right := lipgloss.JoinVertical(lipgloss.Left, rightTop, rightBottom)
	content := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	status := ""
	if m.err != "" {
		status = m.styles.error.Render("Error: " + m.err)
	} else if m.info != "" {
		status = m.styles.info.Render(m.info)
	}

	menu := m.renderMenuPanel()
	form := ""
	if m.form != nil {
		form = m.renderFormPanel()
	}

	parts := []string{header, content, menu}
	if form != "" {
		parts = append(parts, form)
	}
	if status != "" {
		parts = append(parts, status)
	}
	if m.busy {
		parts = append(parts, "Working... (Ctrl+C to quit)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m tuiModel) renderProfilesPanel() string {
	lines := []string{m.styles.label.Render("Profiles")}
	if m.state == nil || len(m.state.Profiles) == 0 {
		lines = append(lines, "No profiles")
	} else {
		for _, p := range m.state.Profiles {
			suffix := ""
			if m.state.Defaults[p.Tool] == p.Name {
				suffix = " (default)"
			}
			lines = append(lines, fmt.Sprintf("- %s/%s%s", p.Tool, p.Name, suffix))
		}
	}
	lines = append(lines, "", m.styles.label.Render("Native Defaults"))
	if len(m.native) == 0 {
		lines = append(lines, "No native defaults detected")
	} else {
		for _, tool := range store.SupportedTools {
			n, ok := m.native[tool]
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s/default", tool))
			lines = append(lines, fmt.Sprintf("    cfg:  %s", n.ConfigDir))
			lines = append(lines, fmt.Sprintf("    sess: %s", n.SessionDir))
		}
	}
	return m.styles.panel.Width(54).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderPresetsPanel() string {
	lines := []string{m.styles.label.Render("Settings Presets")}
	if len(m.presets) == 0 {
		lines = append(lines, "No presets")
	} else {
		for _, p := range m.presets {
			lines = append(lines, fmt.Sprintf("- %s/%s", p.Tool, p.Name))
		}
	}
	return m.styles.panel.Width(58).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderSyncPanel() string {
	lines := []string{m.styles.label.Render("Sync Mappings")}
	if len(m.sync) == 0 {
		lines = append(lines, "No sync mappings")
	} else {
		for _, s := range m.sync {
			lines = append(lines, fmt.Sprintf("- %s/%s -> %s", s.Tool, s.Profile, s.Preset))
		}
	}
	return m.styles.panel.Width(58).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderMenuPanel() string {
	lines := []string{m.styles.label.Render("Actions"), "Use up/down + enter"}
	for i, item := range m.menu {
		line := fmt.Sprintf("%s - %s", item.Title, item.Hint)
		if i == m.cursor && m.form == nil {
			lines = append(lines, m.styles.menuActive.Render("> "+line))
		} else {
			lines = append(lines, m.styles.menuItem.Render("  "+line))
		}
	}
	return m.styles.panel.Width(116).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderFormPanel() string {
	lines := []string{
		m.styles.label.Render(m.form.Title),
		m.form.Hint,
		"",
	}
	for i, input := range m.form.Inputs {
		prefix := "  "
		if i == m.form.Focus {
			prefix = "> "
		}
		lines = append(lines, prefix+m.form.Fields[i]+": "+input.View())
	}
	lines = append(lines, "", "Enter submits on last field. Esc cancels.")
	return m.styles.panel.Width(116).Render(strings.Join(lines, "\n"))
}

func newTUIForm(action tuiAction) *tuiForm {
	newInput := func(placeholder string) textinput.Model {
		in := textinput.New()
		in.Placeholder = placeholder
		in.CharLimit = 128
		in.Width = 40
		return in
	}

	switch action {
	case tuiActionAddProfile:
		fields := []string{"tool", "profile", "share_sessions"}
		inputs := []textinput.Model{
			newInput("codex or claude"),
			newInput("personal"),
			newInput("yes/no (default yes)"),
		}
		inputs[2].SetValue("yes")
		inputs[0].Focus()
		return &tuiForm{
			Action: action,
			Title:  "Add Profile",
			Hint:   "Create a profile and optionally link shared sessions.",
			Fields: fields,
			Inputs: inputs,
		}
	case tuiActionSetDefault:
		fields := []string{"tool", "profile"}
		inputs := []textinput.Model{newInput("codex or claude"), newInput("profile name")}
		inputs[0].Focus()
		return &tuiForm{
			Action: action,
			Title:  "Set Default",
			Hint:   "Set the default profile for a tool.",
			Fields: fields,
			Inputs: inputs,
		}
	case tuiActionSnapshot:
		fields := []string{"tool", "source_profile", "preset"}
		inputs := []textinput.Model{newInput("codex or claude"), newInput("profile name or default"), newInput("preset name")}
		inputs[0].Focus()
		return &tuiForm{
			Action: action,
			Title:  "Snapshot Settings",
			Hint:   "Capture tool-native settings from source profile/default into preset.",
			Fields: fields,
			Inputs: inputs,
		}
	case tuiActionApply:
		fields := []string{"tool", "preset", "target_profile"}
		inputs := []textinput.Model{newInput("codex or claude"), newInput("preset name"), newInput("profile name or default")}
		inputs[0].Focus()
		return &tuiForm{
			Action: action,
			Title:  "Apply Preset",
			Hint:   "Apply a settings preset to one profile.",
			Fields: fields,
			Inputs: inputs,
		}
	case tuiActionEnableSync:
		fields := []string{"tool", "preset", "profile"}
		inputs := []textinput.Model{newInput("codex or claude"), newInput("preset name"), newInput("profile name or default")}
		inputs[0].Focus()
		return &tuiForm{
			Action: action,
			Title:  "Enable Sync",
			Hint:   "Attach a profile to a preset and keep it synced.",
			Fields: fields,
			Inputs: inputs,
		}
	case tuiActionDisableSync:
		fields := []string{"tool", "profile"}
		inputs := []textinput.Model{newInput("codex or claude"), newInput("profile name or default")}
		inputs[0].Focus()
		return &tuiForm{
			Action: action,
			Title:  "Disable Sync",
			Hint:   "Remove preset sync binding from a profile.",
			Fields: fields,
			Inputs: inputs,
		}
	default:
		return nil
	}
}

func tuiRefreshCmd(rootDir string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiErrorMsg{Err: err}
		}
		st, err := mgr.Load()
		if err != nil {
			return tuiErrorMsg{Err: err}
		}
		presets, syncs, err := mgr.ListSettings(nil)
		if err != nil {
			return tuiErrorMsg{Err: err}
		}
		native := map[store.Tool]tuiNativeInfo{}
		for _, tool := range store.SupportedTools {
			cfg, cfgErr := mgr.NativeConfigDir(tool)
			sess, sessErr := mgr.NativeSessionDir(tool)
			if cfgErr != nil || sessErr != nil {
				continue
			}
			native[tool] = tuiNativeInfo{
				ConfigDir:  cfg,
				SessionDir: sess,
			}
		}
		return tuiDataMsg{
			State:   st,
			Presets: presets,
			Sync:    syncs,
			Native:  native,
		}
	}
}

func tuiRunActionCmd(rootDir string, action tuiAction, values map[string]string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiErrorMsg{Err: err}
		}

		switch action {
		case tuiActionAddProfile:
			tool, err := parseTool(values["tool"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			name := values["profile"]
			profile, created, err := mgr.EnsureProfile(tool, name)
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			if !created {
				return tuiDoneMsg{Message: fmt.Sprintf("Profile already exists: %s/%s", tool, name)}
			}
			share := strings.TrimSpace(strings.ToLower(values["share_sessions"]))
			if share == "" || share == "y" || share == "yes" || share == "true" || share == "1" {
				if _, err := mgr.EnableSharedSessions(profile); err != nil {
					return tuiErrorMsg{Err: err}
				}
			}
			if _, err := installShimForProfile(profile); err != nil {
				return tuiDoneMsg{Message: fmt.Sprintf("Profile created (%s/%s), shim install warning: %v", tool, name, err)}
			}
			return tuiDoneMsg{Message: fmt.Sprintf("Profile created: %s/%s", tool, name)}

		case tuiActionSetDefault:
			tool, err := parseTool(values["tool"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			if err := mgr.SetDefault(tool, values["profile"]); err != nil {
				return tuiErrorMsg{Err: err}
			}
			return tuiDoneMsg{Message: fmt.Sprintf("Default set: %s -> %s", tool, values["profile"])}

		case tuiActionSnapshot:
			tool, err := parseTool(values["tool"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			updated, err := mgr.SnapshotSettings(tool, values["source_profile"], values["preset"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			return tuiDoneMsg{Message: fmt.Sprintf("Snapshot updated: %s/%s (synced profiles updated: %d)", tool, values["preset"], updated)}

		case tuiActionApply:
			tool, err := parseTool(values["tool"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			if err := mgr.ApplySettingsPreset(tool, values["preset"], values["target_profile"]); err != nil {
				return tuiErrorMsg{Err: err}
			}
			return tuiDoneMsg{Message: fmt.Sprintf("Applied %s/%s -> %s", tool, values["preset"], values["target_profile"])}

		case tuiActionEnableSync:
			tool, err := parseTool(values["tool"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			if err := mgr.SetSettingsSync(tool, values["profile"], values["preset"], true); err != nil {
				return tuiErrorMsg{Err: err}
			}
			return tuiDoneMsg{Message: fmt.Sprintf("Sync enabled: %s/%s -> %s", tool, values["profile"], values["preset"])}

		case tuiActionDisableSync:
			tool, err := parseTool(values["tool"])
			if err != nil {
				return tuiErrorMsg{Err: err}
			}
			if err := mgr.SetSettingsSync(tool, values["profile"], "", false); err != nil {
				return tuiErrorMsg{Err: err}
			}
			return tuiDoneMsg{Message: fmt.Sprintf("Sync disabled: %s/%s", tool, values["profile"])}
		}

		return tuiErrorMsg{Err: fmt.Errorf("unsupported action")}
	}
}
