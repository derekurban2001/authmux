package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/derekurban/profilex-cli/internal/shim"
	"github.com/derekurban/profilex-cli/internal/store"
	"github.com/derekurban/profilex-cli/internal/usage"
)

type sidebarKind int

const (
	sidebarAdd sidebarKind = iota
	sidebarExport
	sidebarTemplates
	sidebarProfile
	sidebarHeading
)

type sidebarItem struct {
	Kind       sidebarKind
	Label      string
	Selectable bool
	Tool       store.Tool
	Profile    string
}

type tuiDataMsg struct {
	State         *store.State
	Presets       []store.SettingsPreset
	SyncByProfile map[string]string
	SessionShared map[string]bool
	SkillsShared  map[string]bool
}

type tuiOpMsg struct {
	Err     error
	Info    string
	Refresh bool
}

func (m tuiOpMsg) IsError() bool { return m.Err != nil }

type exportResultMsg struct {
	Err    error
	Result exportResult
}

type exportTickMsg struct{}
type statusClearMsg struct{}

type exportResult struct {
	Out      string
	Profiles int
	Events   int
	Roots    int
	Files    int
	Duration time.Duration
}

type modeKind int

const (
	modeNormal modeKind = iota
	modeTemplateApply
	modeTemplateRename
	modeTemplateDelete
	modeProfileRename
	modeProfileDelete
)

type model struct {
	rootDir string
	width   int
	height  int

	state         *store.State
	presets       []store.SettingsPreset
	syncByProfile map[string]string
	sessionShared map[string]bool
	skillsShared  map[string]bool

	sidebar []sidebarItem
	cursor  int

	welcomeActive bool
	wizardStep    int // -1=inactive, 0=tool, 1=name, 2=options, 3=confirm

	addToolIdx   int
	addNameInput textinput.Model
	addShare     bool
	addSync      bool
	addSkills    bool

	exportPathInput textinput.Model
	exportRunning   bool
	exportStarted   time.Time
	exportElapsed   time.Duration
	lastExport      *exportResult

	templateCursor int
	templateSource int
	templateName   textinput.Model

	mode       modeKind
	prompt     textinput.Model
	applyIndex int

	statusMsg     string
	statusIsError bool
	statusExpiry  time.Time
}

func cmdTUI(rootDir string, args []string) error {
	if hasHelp(args) {
		fmt.Printf("Usage: profilex tui\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %s", strings.Join(args, " "))
	}
	p := tea.NewProgram(newModel(rootDir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(rootDir string) model {
	add := textinput.New()
	add.Placeholder = "profile-name"
	add.CharLimit = 64
	add.Width = 28
	add.Focus()

	out := textinput.New()
	out.SetValue("profilex-usage-" + time.Now().Format("20060102-150405") + ".json")
	out.Width = 64

	tpl := textinput.New()
	tpl.Placeholder = "template-name"
	tpl.CharLimit = 64
	tpl.Width = 24

	p := textinput.New()
	p.CharLimit = 64
	p.Width = 28

	return model{
		rootDir:         rootDir,
		width:           120,
		height:          24,
		syncByProfile:   map[string]string{},
		sessionShared:   map[string]bool{},
		skillsShared:    map[string]bool{},
		welcomeActive:   true,
		wizardStep:      -1,
		addShare:        true,
		addSync:         true,
		addSkills:       true,
		addNameInput:    add,
		exportPathInput: out,
		templateName:    tpl,
		prompt:          p,
		templateSource:  0,
		templateCursor:  0,
		addToolIdx:      0,
		mode:            modeNormal,
	}
}

func (m model) Init() tea.Cmd { return refreshCmd(m.rootDir) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		return m, nil
	case tuiDataMsg:
		m.state, m.presets, m.syncByProfile, m.sessionShared, m.skillsShared = msg.State, msg.Presets, msg.SyncByProfile, msg.SessionShared, msg.SkillsShared
		m.sidebar = buildSidebar(msg.State)
		m.cursor = selectableCursor(m.sidebar, m.cursor)
		if m.templateCursor >= len(m.presets) {
			m.templateCursor = max(0, len(m.presets)-1)
		}
		srcOpts := m.sourceProfilesForCreate()
		if m.templateSource >= len(srcOpts) {
			m.templateSource = max(0, len(srcOpts)-1)
		}
		applyOpts := m.applyTargets()
		if m.applyIndex >= len(applyOpts) {
			m.applyIndex = max(0, len(applyOpts)-1)
		}
		// Auto-dismiss welcome screen if profiles exist
		if m.welcomeActive && msg.State != nil && len(msg.State.Profiles) > 0 {
			m.welcomeActive = false
			m.wizardStep = -1
		}
		return m, nil
	case tuiOpMsg:
		m.mode = modeNormal
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
			m.statusIsError = true
		} else if msg.Info != "" {
			m.statusMsg = msg.Info
			m.statusIsError = false
		}
		m.statusExpiry = time.Now().Add(5 * time.Second)
		var cmds []tea.Cmd
		if msg.Refresh {
			cmds = append(cmds, refreshCmd(m.rootDir))
		}
		cmds = append(cmds, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} }))
		if msg.Refresh && !msg.IsError() && m.wizardStep >= 0 {
			m.wizardStep = -1
			m.addNameInput.SetValue("")
		}
		return m, tea.Batch(cmds...)
	case statusClearMsg:
		if !m.statusExpiry.IsZero() && time.Now().After(m.statusExpiry) {
			m.statusMsg = ""
		}
		return m, nil
	case exportResultMsg:
		m.exportRunning = false
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
			m.statusIsError = true
		} else {
			m.lastExport = &msg.Result
			m.statusMsg = "Usage export complete"
			m.statusIsError = false
		}
		m.statusExpiry = time.Now().Add(5 * time.Second)
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} })
	case exportTickMsg:
		if m.exportRunning {
			m.exportElapsed = time.Since(m.exportStarted)
			return m, exportTick()
		}
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// Welcome screen: only Enter or q
	if m.welcomeActive {
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "enter":
			m.welcomeActive = false
			m.wizardStep = 0
			// Move cursor to Add Profile
			for i, it := range m.sidebar {
				if it.Kind == sidebarAdd {
					m.cursor = i
					break
				}
			}
			return m, nil
		}
		return m, nil
	}

	if key.String() == "q" {
		// Don't quit while typing in text inputs
		if m.wizardStep == 1 {
			return m, nil
		}
		return m, tea.Quit
	}

	// Wizard navigation
	if m.wizardStep >= 0 {
		return m.updateAddWizard(key)
	}

	if m.mode != modeNormal {
		return m.updateMode(key)
	}

	switch key.String() {
	case "up", "k":
		m.cursor = moveCursor(m.sidebar, m.cursor, -1)
		return m, nil
	case "down", "j":
		m.cursor = moveCursor(m.sidebar, m.cursor, 1)
		return m, nil
	}

	switch m.selected().Kind {
	case sidebarAdd:
		return m.updateAdd(key)
	case sidebarExport:
		return m.updateExport(key)
	case sidebarTemplates:
		return m.updateTemplates(key)
	case sidebarProfile:
		return m.updateProfile(key)
	default:
		return m, nil
	}
}

func (m model) updateAdd(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When Add Profile is selected and no wizard is active, start the wizard
	if key.String() == "enter" && m.wizardStep < 0 {
		m.wizardStep = 0
		m.addToolIdx = 0
		m.addNameInput.SetValue("")
		m.addShare = true
		m.addSync = true
		m.addSkills = true
		return m, nil
	}
	return m, nil
}

func (m model) updateAddWizard(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.wizardStep {
	case 0: // Tool selection
		switch key.String() {
		case "up", "k":
			m.addToolIdx = (m.addToolIdx + len(store.SupportedTools) - 1) % len(store.SupportedTools)
			return m, nil
		case "down", "j":
			m.addToolIdx = (m.addToolIdx + 1) % len(store.SupportedTools)
			return m, nil
		case "enter":
			m.wizardStep = 1
			m.addNameInput.Focus()
			return m, nil
		case "esc":
			m.wizardStep = -1
			return m, nil
		}
		return m, nil
	case 1: // Name input
		switch key.String() {
		case "enter":
			name := strings.TrimSpace(m.addNameInput.Value())
			if name == "" {
				m.statusMsg = "profile name is required"
				m.statusIsError = true
				m.statusExpiry = time.Now().Add(5 * time.Second)
				return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} })
			}
			m.wizardStep = 2
			return m, nil
		case "esc":
			m.wizardStep = 0
			return m, nil
		}
		var cmd tea.Cmd
		m.addNameInput, cmd = m.addNameInput.Update(key)
		return m, cmd
	case 2: // Options
		switch key.String() {
		case "s":
			m.addShare = !m.addShare
			return m, nil
		case "c":
			m.addSync = !m.addSync
			return m, nil
		case "k":
			m.addSkills = !m.addSkills
			return m, nil
		case "enter":
			m.wizardStep = 3
			return m, nil
		case "esc":
			m.wizardStep = 1
			m.addNameInput.Focus()
			return m, nil
		}
		return m, nil
	case 3: // Confirm
		switch key.String() {
		case "enter":
			name := strings.TrimSpace(m.addNameInput.Value())
			return m, addProfileCmd(m.rootDir, store.SupportedTools[m.addToolIdx], name, m.addShare, m.addSync, m.addSkills)
		case "esc":
			m.wizardStep = 2
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m model) updateExport(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.exportRunning {
		return m, nil
	}
	if key.String() == "enter" {
		out := strings.TrimSpace(m.exportPathInput.Value())
		if out == "" {
			out = "profilex-usage-" + time.Now().Format("20060102-150405") + ".json"
			m.exportPathInput.SetValue(out)
		}
		m.exportRunning, m.exportStarted, m.exportElapsed = true, time.Now(), 0
		return m, tea.Batch(exportCmd(m.rootDir, out), exportTick())
	}
	var cmd tea.Cmd
	m.exportPathInput, cmd = m.exportPathInput.Update(key)
	return m, cmd
}

func (m model) updateTemplates(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "[":
		if len(m.presets) > 0 {
			m.templateCursor = (m.templateCursor + len(m.presets) - 1) % len(m.presets)
		}
		return m, nil
	case "]":
		if len(m.presets) > 0 {
			m.templateCursor = (m.templateCursor + 1) % len(m.presets)
		}
		return m, nil
	case ",":
		opts := m.sourceProfilesForCreate()
		if len(opts) > 0 {
			m.templateSource = (m.templateSource + len(opts) - 1) % len(opts)
		}
		return m, nil
	case ".":
		opts := m.sourceProfilesForCreate()
		if len(opts) > 0 {
			m.templateSource = (m.templateSource + 1) % len(opts)
		}
		return m, nil
	case "enter":
		opts := m.sourceProfilesForCreate()
		if len(opts) == 0 {
			m.statusMsg, m.statusIsError = "no source profiles available", true
			m.statusExpiry = time.Now().Add(5 * time.Second)
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} })
		}
		name := strings.TrimSpace(m.templateName.Value())
		if name == "" {
			m.statusMsg, m.statusIsError = "template name is required", true
			m.statusExpiry = time.Now().Add(5 * time.Second)
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} })
		}
		return m, createTemplateCmd(m.rootDir, m.templateTool(), opts[m.templateSource], name)
	case "a":
		if len(m.presets) == 0 {
			return m, nil
		}
		m.applyIndex = 0
		m.mode = modeTemplateApply
		return m, nil
	case "r":
		if len(m.presets) == 0 {
			return m, nil
		}
		m.prompt.SetValue(m.presets[m.templateCursor].Name)
		m.prompt.Focus()
		m.mode = modeTemplateRename
		return m, nil
	case "d", "x":
		if len(m.presets) == 0 {
			return m, nil
		}
		m.mode = modeTemplateDelete
		return m, nil
	}
	var cmd tea.Cmd
	m.templateName, cmd = m.templateName.Update(key)
	return m, cmd
}

func (m model) updateProfile(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	item := m.selected()
	switch key.String() {
	case "s":
		return m, toggleSessionCmd(m.rootDir, item.Tool, item.Profile, m.sessionShared[pk(item.Tool, item.Profile)])
	case "c":
		_, synced := m.syncByProfile[pk(item.Tool, item.Profile)]
		return m, toggleConfigSyncCmd(m.rootDir, item.Tool, item.Profile, synced)
	case "k":
		return m, toggleSkillsCmd(m.rootDir, item.Tool, item.Profile, m.skillsShared[pk(item.Tool, item.Profile)])
	case "r":
		m.prompt.SetValue(item.Profile)
		m.prompt.Focus()
		m.mode = modeProfileRename
		return m, nil
	case "d", "x":
		m.mode = modeProfileDelete
		return m, nil
	}
	return m, nil
}

func (m model) updateMode(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "esc" {
		m.mode = modeNormal
		return m, nil
	}
	switch m.mode {
	case modeTemplateApply:
		opts := m.applyTargets()
		switch key.String() {
		case "up", "k":
			if len(opts) > 0 {
				m.applyIndex = (m.applyIndex + len(opts) - 1) % len(opts)
			}
		case "down", "j":
			if len(opts) > 0 {
				m.applyIndex = (m.applyIndex + 1) % len(opts)
			}
		case "enter":
			if len(opts) > 0 {
				t := m.presets[m.templateCursor]
				return m, applyTemplateCmd(m.rootDir, t.Tool, t.Name, opts[m.applyIndex])
			}
		}
		return m, nil
	case modeTemplateRename:
		if key.String() == "enter" {
			newName := strings.TrimSpace(m.prompt.Value())
			if newName == "" {
				m.statusMsg, m.statusIsError = "new template name is required", true
				m.statusExpiry = time.Now().Add(5 * time.Second)
				return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} })
			}
			t := m.presets[m.templateCursor]
			return m, renameTemplateCmd(m.rootDir, t.Tool, t.Name, newName)
		}
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(key)
		return m, cmd
	case modeTemplateDelete:
		if strings.ToLower(key.String()) == "y" || key.String() == "enter" {
			t := m.presets[m.templateCursor]
			return m, deleteTemplateCmd(m.rootDir, t.Tool, t.Name)
		}
		if strings.ToLower(key.String()) == "n" {
			m.mode = modeNormal
		}
		return m, nil
	case modeProfileRename:
		if key.String() == "enter" {
			item := m.selected()
			newName := strings.TrimSpace(m.prompt.Value())
			if newName == "" {
				m.statusMsg, m.statusIsError = "new profile name is required", true
				m.statusExpiry = time.Now().Add(5 * time.Second)
				return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return statusClearMsg{} })
			}
			return m, renameProfileCmd(m.rootDir, item.Tool, item.Profile, newName)
		}
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(key)
		return m, cmd
	case modeProfileDelete:
		if strings.ToLower(key.String()) == "y" || key.String() == "enter" {
			item := m.selected()
			return m, deleteProfileCmd(m.rootDir, item.Tool, item.Profile)
		}
		if strings.ToLower(key.String()) == "n" {
			m.mode = modeNormal
		}
	}
	return m, nil
}

func (m model) View() string {
	// Welcome screen (full-screen centered)
	if m.welcomeActive && (m.state == nil || len(m.state.Profiles) == 0) {
		return m.renderWelcome()
	}

	h := m.renderHeader()
	side := m.renderSidebar()
	main := m.renderMain()
	helpBar := m.renderHelpBar()

	body := lipgloss.JoinHorizontal(lipgloss.Top, side, main)
	out := h + "\n" + body

	// Timed status message
	if m.statusMsg != "" {
		if m.statusIsError {
			out += "\n" + styleError.Render("Error: "+m.statusMsg)
		} else {
			out += "\n" + styleSuccess.Render(m.statusMsg)
		}
	}

	out += "\n" + helpBar
	return out
}

func (m model) renderWelcome() string {
	w := max(60, m.width-20)
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(2, 4).
		Width(w)

	title := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("Welcome to ProfileX")
	badge1 := renderToolBadge(store.ToolClaude)
	badge2 := renderToolBadge(store.ToolCodex)

	content := strings.Join([]string{
		title,
		"",
		"ProfileX manages isolated profiles for your AI coding tools.",
		"Each profile gets its own sessions, settings, and skills.",
		"",
		"Supported tools:  " + badge1 + "  " + badge2,
		"",
		"Example: creating a profile named " + styleSectionTitle.Render("work") +
			" gives you the command " + styleSectionTitle.Render("claude-work"),
		"",
		renderDivider(w - 12),
		"",
		styleSuccess.Render("Press Enter to create your first profile"),
		styleMuted.Render("q to quit"),
	}, "\n")

	rendered := box.Render(content)

	// Center vertically
	pad := (m.height - lipgloss.Height(rendered)) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat("\n", pad) + rendered
}

func (m model) renderHeader() string {
	ver := resolvedVersion()
	info := fmt.Sprintf("ProfileX v%s", ver)

	if m.state != nil && len(m.state.Profiles) > 0 {
		claudeCount, codexCount := 0, 0
		for _, p := range m.state.Profiles {
			if p.Tool == store.ToolClaude {
				claudeCount++
			} else {
				codexCount++
			}
		}
		parts := []string{fmt.Sprintf("%d profiles", len(m.state.Profiles))}
		if claudeCount > 0 {
			parts = append(parts, fmt.Sprintf("%d claude", claudeCount))
		}
		if codexCount > 0 {
			parts = append(parts, fmt.Sprintf("%d codex", codexCount))
		}
		info += " | " + strings.Join(parts, ", ")
	}

	info += " | q quit"
	return styleHeader.Render(info)
}

func (m model) renderSidebar() string {
	s := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorCardBorder).
		Padding(1, 1).
		Width(34)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0F172A")).
		Background(colorAccent)

	lines := []string{styleSectionTitle.Render("Navigation")}
	for i, it := range m.sidebar {
		if !it.Selectable {
			if it.Kind == sidebarHeading {
				label := it.Label
				if label == "(none)" {
					lines = append(lines, "")
					lines = append(lines, styleMuted.Render("  No profiles yet"))
					continue
				}
				if label == "Profiles" {
					lines = append(lines, "")
					lines = append(lines, styleSectionTitle.Render("  PROFILES"))
					continue
				}
				// Tool heading -- use badge
				tool, ok := store.IsSupportedTool(label)
				if ok {
					lines = append(lines, "  "+renderToolBadge(tool))
				} else {
					lines = append(lines, "  "+strings.ToUpper(label))
				}
				continue
			}
			continue
		}

		label := it.Label
		prefix := "  "
		switch it.Kind {
		case sidebarAdd:
			label = "+ Add Profile"
		case sidebarExport:
			label = "^ Export Usage"
		case sidebarTemplates:
			label = "# Templates"
		case sidebarProfile:
			// Mark default profiles with *
			if m.state != nil {
				if def, ok := m.state.Defaults[it.Tool]; ok && def == it.Profile {
					label = "  " + it.Profile + " *"
				} else {
					label = "  " + it.Profile
				}
			}
		}

		if i == m.cursor {
			lines = append(lines, cursorStyle.Render("> "+label))
		} else {
			lines = append(lines, prefix+label)
		}
	}
	return s.Render(strings.Join(lines, "\n"))
}

func (m model) renderMain() string {
	mainW := max(60, m.width-38)
	s := styleCard.Width(mainW)

	// If wizard is active, render that instead
	if m.wizardStep >= 0 {
		return s.Render(m.renderWizard(mainW))
	}

	var content string
	switch m.selected().Kind {
	case sidebarAdd:
		content = m.renderAddPanel()
	case sidebarExport:
		content = m.renderExportPanel(mainW)
	case sidebarTemplates:
		content = m.renderTemplatesPanel(mainW)
	case sidebarProfile:
		content = m.renderProfileCard(mainW)
	default:
		content = styleSectionTitle.Render("ProfileX") + "\n\n" +
			styleMuted.Render("Select a sidebar item to get started.")
	}

	// Mode overlay
	if m.mode != modeNormal {
		content += "\n\n" + m.renderModeOverlay()
	}

	return s.Render(content)
}

func (m model) renderAddPanel() string {
	title := styleSectionTitle.Render("+ Add Profile")
	return title + "\n\n" +
		styleMuted.Render("Press Enter to start the profile creation wizard.")
}

func (m model) renderWizard(w int) string {
	divW := w - 8
	if divW < 20 {
		divW = 20
	}
	steps := []string{"Tool", "Name", "Options", "Confirm"}
	stepLine := ""
	for i, name := range steps {
		if i == m.wizardStep {
			stepLine += styleSuccess.Render(fmt.Sprintf("[%d %s]", i+1, name))
		} else if i < m.wizardStep {
			stepLine += styleMuted.Render(fmt.Sprintf(" %d %s ", i+1, name))
		} else {
			stepLine += styleMuted.Render(fmt.Sprintf(" %d %s ", i+1, name))
		}
		if i < len(steps)-1 {
			stepLine += styleMuted.Render(" > ")
		}
	}

	title := styleSectionTitle.Render("Create Profile") + "\n" + stepLine + "\n" + renderDivider(divW) + "\n\n"

	switch m.wizardStep {
	case 0:
		lines := []string{title}
		lines = append(lines, "Choose your tool:\n")
		for i, tool := range store.SupportedTools {
			badge := renderToolBadge(tool)
			desc := ""
			if tool == store.ToolClaude {
				desc = "Anthropic's Claude Code CLI"
			} else {
				desc = "OpenAI's Codex CLI"
			}
			cursor := "  "
			if i == m.addToolIdx {
				cursor = "> "
				lines = append(lines, styleSuccess.Render(cursor)+badge+"  "+desc)
			} else {
				lines = append(lines, cursor+badge+"  "+styleMuted.Render(desc))
			}
		}
		lines = append(lines, "", styleMuted.Render("Up/Down to select, Enter to continue, Esc to cancel"))
		return strings.Join(lines, "\n")

	case 1:
		tool := store.SupportedTools[m.addToolIdx]
		return title +
			"Profile name:\n\n" +
			"  " + m.addNameInput.View() + "\n\n" +
			styleMuted.Render(fmt.Sprintf("This creates the command %s-%s", tool, m.addNameInput.Value())) + "\n\n" +
			styleMuted.Render("Enter to continue, Esc to go back")

	case 2:
		return title +
			"Configure options:\n\n" +
			fmt.Sprintf("  %s  Session sharing   %s\n", renderKeyHint("s", "toggle"), renderToggle(m.addShare)) +
			fmt.Sprintf("  %s  Skills sharing    %s\n", renderKeyHint("k", "toggle"), renderToggle(m.addSkills)) +
			fmt.Sprintf("  %s  Config sync       %s\n", renderKeyHint("c", "toggle"), renderToggle(m.addSync)) +
			"\n" + styleMuted.Render("Enter to continue, Esc to go back")

	case 3:
		tool := store.SupportedTools[m.addToolIdx]
		name := strings.TrimSpace(m.addNameInput.Value())
		return title +
			"Review and confirm:\n\n" +
			renderDivider(divW) + "\n" +
			"  Tool:             " + renderToolBadge(tool) + "\n" +
			"  Profile name:     " + styleSectionTitle.Render(name) + "\n" +
			"  Command:          " + styleSectionTitle.Render(fmt.Sprintf("%s-%s", tool, name)) + "\n" +
			"  Session sharing:  " + renderToggle(m.addShare) + "\n" +
			"  Skills sharing:   " + renderToggle(m.addSkills) + "\n" +
			"  Config sync:      " + renderToggle(m.addSync) + "\n" +
			renderDivider(divW) + "\n\n" +
			styleSuccess.Render("Press Enter to create") + "  " + styleMuted.Render("Esc to go back")
	}
	return ""
}

func (m model) renderExportPanel(w int) string {
	divW := w - 8
	if divW < 20 {
		divW = 20
	}
	title := styleSectionTitle.Render("^ Export Usage")
	lines := []string{title, ""}
	lines = append(lines, styleMuted.Render("Export a unified usage bundle for analysis in ProfileX-UI."))
	lines = append(lines, "")
	lines = append(lines, "Output file: "+m.exportPathInput.View())

	if m.state != nil {
		lines = append(lines, styleMuted.Render(fmt.Sprintf("Profiles found: %d", len(m.state.Profiles))))
	}

	lines = append(lines, "")

	if m.exportRunning {
		elapsed := m.exportElapsed.Round(100 * time.Millisecond)
		lines = append(lines, styleWarning.Render(fmt.Sprintf("Exporting... %s", elapsed)))
	} else {
		lines = append(lines, renderKeyHint("Enter", "start export"))
	}

	if m.lastExport != nil {
		r := m.lastExport
		lines = append(lines, "", renderDivider(divW), "")
		lines = append(lines, styleSuccess.Render("Last export complete"))
		lines = append(lines, fmt.Sprintf("  File:      %s", r.Out))
		lines = append(lines, fmt.Sprintf("  Duration:  %s", r.Duration.Round(100*time.Millisecond)))
		lines = append(lines, fmt.Sprintf("  Profiles: %d  Events: %d  Roots: %d  Files: %d",
			r.Profiles, r.Events, r.Roots, r.Files))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderTemplatesPanel(w int) string {
	divW := w - 8
	if divW < 20 {
		divW = 20
	}
	title := styleSectionTitle.Render("# Templates")
	lines := []string{title, ""}

	if len(m.presets) == 0 {
		lines = append(lines, styleMuted.Render("No templates yet."))
		lines = append(lines, "")
		lines = append(lines, styleMuted.Render("Templates let you save and reuse settings across profiles."))
		lines = append(lines, styleMuted.Render("Create one below from an existing profile's settings."))
	} else {
		lines = append(lines, styleMuted.Render("Saved templates:"))
		lines = append(lines, "")
		for i, t := range m.presets {
			badge := renderToolBadge(t.Tool)
			cursor := "  "
			if i == m.templateCursor {
				cursor = "> "
			}
			lines = append(lines, cursor+badge+"  "+t.Name)
		}
		lines = append(lines, "")
		lines = append(lines, renderKeyHint("[", "prev")+" "+renderKeyHint("]", "next")+
			"  "+renderKeyHint("a", "apply")+" "+renderKeyHint("r", "rename")+" "+renderKeyHint("d", "delete"))
	}

	lines = append(lines, "", renderDivider(divW), "")
	lines = append(lines, styleSectionTitle.Render("Create new template"))
	lines = append(lines, "")

	opts := m.sourceProfilesForCreate()
	src := "default"
	if len(opts) > 0 {
		src = opts[clampIndex(m.templateSource, len(opts))]
	}
	lines = append(lines, fmt.Sprintf("  Source: %s/%s  ", m.templateTool(), src)+renderKeyHint(",", "prev")+" "+renderKeyHint(".", "next"))
	lines = append(lines, "  Name:   "+m.templateName.View())
	lines = append(lines, "")
	lines = append(lines, "  "+renderKeyHint("Enter", "create template"))

	return strings.Join(lines, "\n")
}

func (m model) renderProfileCard(w int) string {
	divW := w - 8
	if divW < 20 {
		divW = 20
	}
	it := m.selected()
	key := pk(it.Tool, it.Profile)

	badge := renderToolBadge(it.Tool)
	name := lipgloss.NewStyle().Bold(true).Render(it.Profile)

	defaultTag := ""
	if m.state != nil {
		if def, ok := m.state.Defaults[it.Tool]; ok && def == it.Profile {
			defaultTag = "  " + styleMuted.Render("(default)")
		}
	}

	sessionOn := m.sessionShared[key]
	skillsOn := m.skillsShared[key]
	syncPreset := m.syncByProfile[key]
	syncOn := syncPreset != ""

	lines := []string{
		badge + "  " + name + defaultTag,
		renderDivider(divW),
		fmt.Sprintf("  Session sharing   %s    %s", renderToggle(sessionOn), renderKeyHint("s", "toggle")),
		fmt.Sprintf("  Skills sharing    %s    %s", renderToggle(skillsOn), renderKeyHint("k", "toggle")),
	}
	if syncOn {
		lines = append(lines, fmt.Sprintf("  Config sync       %s    %s  %s", renderToggle(true), renderKeyHint("c", "toggle"), styleMuted.Render("("+syncPreset+")")))
	} else {
		lines = append(lines, fmt.Sprintf("  Config sync       %s   %s", renderToggle(false), renderKeyHint("c", "toggle")))
	}
	lines = append(lines, renderDivider(divW))
	lines = append(lines, "  "+renderKeyHint("r", "rename")+"    "+renderKeyHint("d", "delete"))

	return strings.Join(lines, "\n")
}

func (m model) renderModeOverlay() string {
	var content string
	switch m.mode {
	case modeTemplateApply:
		opts := m.applyTargets()
		cur := "default"
		if len(opts) > 0 {
			cur = opts[clampIndex(m.applyIndex, len(opts))]
		}
		content = styleWarning.Render("Apply Template") + "\n\n" +
			fmt.Sprintf("Target profile: %s", styleSectionTitle.Render(cur)) + "\n\n" +
			renderKeyHint("Up/Down", "select") + "  " + renderKeyHint("Enter", "apply") + "  " + renderKeyHint("Esc", "cancel")
	case modeTemplateRename:
		content = styleWarning.Render("Rename Template") + "\n\n" +
			"New name: " + m.prompt.View() + "\n\n" +
			renderKeyHint("Enter", "confirm") + "  " + renderKeyHint("Esc", "cancel")
	case modeTemplateDelete:
		content = styleError.Render("Delete Template") + "\n\n" +
			"Are you sure you want to delete this template?\n\n" +
			renderKeyHint("y", "confirm delete") + "  " + renderKeyHint("n", "cancel") + "  " + renderKeyHint("Esc", "cancel")
	case modeProfileRename:
		content = styleWarning.Render("Rename Profile") + "\n\n" +
			"New name: " + m.prompt.View() + "\n\n" +
			renderKeyHint("Enter", "confirm") + "  " + renderKeyHint("Esc", "cancel")
	case modeProfileDelete:
		content = styleError.Render("Delete Profile") + "\n\n" +
			"Are you sure you want to delete this profile?\n\n" +
			renderKeyHint("y", "confirm delete") + "  " + renderKeyHint("n", "cancel") + "  " + renderKeyHint("Esc", "cancel")
	default:
		return ""
	}
	return styleOverlay.Render(content)
}

func (m model) renderHelpBar() string {
	var hints []string

	if m.wizardStep >= 0 {
		switch m.wizardStep {
		case 0:
			hints = append(hints, renderKeyHint("Up/Down", "select"), renderKeyHint("Enter", "next"), renderKeyHint("Esc", "cancel"))
		case 1:
			hints = append(hints, renderKeyHint("Enter", "next"), renderKeyHint("Esc", "back"))
		case 2:
			hints = append(hints, renderKeyHint("s", "sessions"), renderKeyHint("k", "skills"), renderKeyHint("c", "config"), renderKeyHint("Enter", "next"), renderKeyHint("Esc", "back"))
		case 3:
			hints = append(hints, renderKeyHint("Enter", "create"), renderKeyHint("Esc", "back"))
		}
		hints = append(hints, renderKeyHint("q", "quit"))
		return strings.Join(hints, "  ")
	}

	if m.mode != modeNormal {
		switch m.mode {
		case modeProfileDelete, modeTemplateDelete:
			hints = append(hints, renderKeyHint("y", "confirm delete"), renderKeyHint("n", "cancel"), renderKeyHint("Esc", "cancel"))
		case modeProfileRename, modeTemplateRename:
			hints = append(hints, renderKeyHint("Enter", "confirm"), renderKeyHint("Esc", "cancel"))
		case modeTemplateApply:
			hints = append(hints, renderKeyHint("Up/Down", "select"), renderKeyHint("Enter", "apply"), renderKeyHint("Esc", "cancel"))
		}
		return strings.Join(hints, "  ")
	}

	switch m.selected().Kind {
	case sidebarProfile:
		hints = append(hints, renderKeyHint("s", "sessions"), renderKeyHint("k", "skills"), renderKeyHint("c", "config sync"), renderKeyHint("r", "rename"), renderKeyHint("d", "delete"))
	case sidebarTemplates:
		if len(m.presets) > 0 {
			hints = append(hints, renderKeyHint("[/]", "template"), renderKeyHint("a", "apply"), renderKeyHint("r", "rename"), renderKeyHint("d", "delete"))
		}
		hints = append(hints, renderKeyHint(",/.", "source"), renderKeyHint("Enter", "create"))
	case sidebarExport:
		hints = append(hints, renderKeyHint("Enter", "export"))
	case sidebarAdd:
		hints = append(hints, renderKeyHint("Enter", "start wizard"))
	}
	hints = append(hints, renderKeyHint("q", "quit"))
	return strings.Join(hints, "  ")
}

func (m modeKind) String() string {
	switch m {
	case modeTemplateApply:
		return "template-apply"
	case modeTemplateRename:
		return "template-rename"
	case modeTemplateDelete:
		return "template-delete"
	case modeProfileRename:
		return "profile-rename"
	case modeProfileDelete:
		return "profile-delete"
	default:
		return "normal"
	}
}

func (m model) selected() sidebarItem {
	if m.cursor >= 0 && m.cursor < len(m.sidebar) {
		return m.sidebar[m.cursor]
	}
	return sidebarItem{Kind: sidebarHeading}
}

func (m model) templateTool() store.Tool {
	if len(m.presets) > 0 {
		return m.presets[m.templateCursor].Tool
	}
	return store.SupportedTools[m.addToolIdx]
}

func (m model) sourceProfilesForCreate() []string {
	opts := []string{"default"}
	tool := m.templateTool()
	if m.state == nil {
		return opts
	}
	for _, p := range m.state.Profiles {
		if p.Tool == tool {
			opts = append(opts, p.Name)
		}
	}
	return opts
}

func (m model) applyTargets() []string {
	if len(m.presets) == 0 {
		return nil
	}
	tool := m.presets[m.templateCursor].Tool
	out := []string{"default"}
	if m.state == nil {
		return out
	}
	for _, p := range m.state.Profiles {
		if p.Tool == tool {
			out = append(out, p.Name)
		}
	}
	return out
}

func buildSidebar(st *store.State) []sidebarItem {
	items := []sidebarItem{
		{Kind: sidebarAdd, Label: "Add Profile", Selectable: true},
		{Kind: sidebarExport, Label: "Export Usage", Selectable: true},
		{Kind: sidebarTemplates, Label: "Templates", Selectable: true},
		{Kind: sidebarHeading, Label: "Profiles", Selectable: false},
	}
	if st == nil || len(st.Profiles) == 0 {
		items = append(items, sidebarItem{Kind: sidebarHeading, Label: "(none)", Selectable: false})
		return items
	}
	for _, t := range store.SupportedTools {
		items = append(items, sidebarItem{Kind: sidebarHeading, Label: string(t), Selectable: false})
		for _, p := range st.Profiles {
			if p.Tool == t {
				items = append(items, sidebarItem{Kind: sidebarProfile, Label: "  " + p.Name, Selectable: true, Tool: p.Tool, Profile: p.Name})
			}
		}
	}
	return items
}

func selectableCursor(items []sidebarItem, cur int) int {
	if cur >= 0 && cur < len(items) && items[cur].Selectable {
		return cur
	}
	for i := range items {
		if items[i].Selectable {
			return i
		}
	}
	return 0
}

func moveCursor(items []sidebarItem, cur, step int) int {
	if len(items) == 0 {
		return 0
	}
	i := cur
	for n := 0; n < len(items); n++ {
		i += step
		if i < 0 {
			i = len(items) - 1
		}
		if i >= len(items) {
			i = 0
		}
		if items[i].Selectable {
			return i
		}
	}
	return cur
}

func pk(tool store.Tool, profile string) string { return string(tool) + "/" + profile }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampIndex(idx, n int) int {
	if n <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}

func refreshCmd(rootDir string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		st, err := mgr.Load()
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		presets, syncs, err := mgr.ListSettings(nil)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		syncBy := map[string]string{}
		for _, s := range syncs {
			syncBy[pk(s.Tool, s.Profile)] = s.Preset
		}
		shared := map[string]bool{}
		skills := map[string]bool{}
		for _, p := range st.Profiles {
			prof, e := mgr.GetProfile(st, p.Tool, p.Name)
			if e != nil {
				continue
			}
			on, e := mgr.SharedSessionsEnabled(prof)
			if e == nil {
				shared[pk(prof.Tool, prof.Name)] = on
			}
			skillsOn, e := mgr.SharedSkillsEnabled(prof)
			if e == nil {
				skills[pk(prof.Tool, prof.Name)] = skillsOn
			}
		}
		return tuiDataMsg{State: st, Presets: presets, SyncByProfile: syncBy, SessionShared: shared, SkillsShared: skills}
	}
}

func addProfileCmd(rootDir string, tool store.Tool, name string, share, sync, shareSkills bool) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		p, created, err := mgr.EnsureProfile(tool, name)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if !created {
			return tuiOpMsg{Info: "Profile already exists", Refresh: true}
		}
		if share {
			if _, err := mgr.EnableSharedSessions(p); err != nil {
				return tuiOpMsg{Err: err}
			}
		}
		if shareSkills {
			if _, err := mgr.EnableSharedSkills(p); err != nil {
				return tuiOpMsg{Err: err}
			}
		}
		if sync {
			tpl := fmt.Sprintf("default-%s", tool)
			if _, err := mgr.SnapshotSettings(tool, "default", tpl); err != nil {
				return tuiOpMsg{Err: err}
			}
			if err := mgr.SetSettingsSync(tool, name, tpl, true); err != nil {
				return tuiOpMsg{Err: err}
			}
		}
		_, _ = installShimForProfile(p)
		return tuiOpMsg{Info: fmt.Sprintf("Created %s/%s", tool, name), Refresh: true}
	}
}

func toggleSessionCmd(rootDir string, tool store.Tool, profile string, currentlyShared bool) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		st, err := mgr.Load()
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		p, err := mgr.GetProfile(st, tool, profile)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if currentlyShared {
			err = mgr.DisableSharedSessions(p)
		} else {
			_, err = mgr.EnableSharedSessions(p)
		}
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Session sharing updated", Refresh: true}
	}
}

func toggleSkillsCmd(rootDir string, tool store.Tool, profile string, currentlyShared bool) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		st, err := mgr.Load()
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		p, err := mgr.GetProfile(st, tool, profile)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if currentlyShared {
			err = mgr.DisableSharedSkills(p)
		} else {
			_, err = mgr.EnableSharedSkills(p)
		}
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Skills sharing updated", Refresh: true}
	}
}

func toggleConfigSyncCmd(rootDir string, tool store.Tool, profile string, currentlySynced bool) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if currentlySynced {
			err = mgr.SetSettingsSync(tool, profile, "", false)
		} else {
			tpl := fmt.Sprintf("default-%s", tool)
			if _, err := mgr.SnapshotSettings(tool, "default", tpl); err != nil {
				return tuiOpMsg{Err: err}
			}
			err = mgr.SetSettingsSync(tool, profile, tpl, true)
		}
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Config sync updated", Refresh: true}
	}
}

func renameProfileCmd(rootDir string, tool store.Tool, oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if dir, _ := shim.DefaultShimDir(); dir != "" {
			_ = shim.Remove(dir, store.Profile{Tool: tool, Name: oldName})
		}
		if err := mgr.RenameProfile(tool, oldName, newName); err != nil {
			return tuiOpMsg{Err: err}
		}
		if st, err := mgr.Load(); err == nil {
			if _, p := store.FindProfile(st, tool, newName); p != nil {
				_, _ = installShimForProfile(*p)
			}
		}
		return tuiOpMsg{Info: "Profile renamed", Refresh: true}
	}
}

func deleteProfileCmd(rootDir string, tool store.Tool, name string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if dir, _ := shim.DefaultShimDir(); dir != "" {
			_ = shim.Remove(dir, store.Profile{Tool: tool, Name: name})
		}
		if err := mgr.RemoveProfile(tool, name, false); err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Profile deleted", Refresh: true}
	}
}

func createTemplateCmd(rootDir string, tool store.Tool, source, name string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if _, err := mgr.SnapshotSettings(tool, source, name); err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Template created", Refresh: true}
	}
}

func applyTemplateCmd(rootDir string, tool store.Tool, template, target string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if err := mgr.ApplySettingsPreset(tool, template, target); err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Template applied", Refresh: true}
	}
}

func renameTemplateCmd(rootDir string, tool store.Tool, oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if err := mgr.RenameSettingsPreset(tool, oldName, newName); err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Template renamed", Refresh: true}
	}
}

func deleteTemplateCmd(rootDir string, tool store.Tool, name string) tea.Cmd {
	return func() tea.Msg {
		mgr, err := newManager(rootDir)
		if err != nil {
			return tuiOpMsg{Err: err}
		}
		if err := mgr.DeleteSettingsPreset(tool, name); err != nil {
			return tuiOpMsg{Err: err}
		}
		return tuiOpMsg{Info: "Template deleted", Refresh: true}
	}
}

func exportCmd(rootDir, outPath string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		mgr, err := newManager(rootDir)
		if err != nil {
			return exportResultMsg{Err: err}
		}
		st, err := mgr.Load()
		if err != nil {
			return exportResultMsg{Err: err}
		}
		resolvedRoot, err := resolveRootDir(rootDir)
		if err != nil {
			return exportResultMsg{Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		bundle, err := usage.GenerateBundle(ctx, st, filepath.Join(resolvedRoot, "state.json"), usage.GenerateOptions{
			RootDir:  resolvedRoot,
			Deep:     true,
			MaxFiles: 5000,
			Timezone: time.Now().Location().String(),
			CostMode: usage.CostModeAuto,
		})
		if err != nil {
			return exportResultMsg{Err: err}
		}
		if err := usage.WriteBundle(outPath, bundle); err != nil {
			return exportResultMsg{Err: err}
		}
		return exportResultMsg{Result: exportResult{
			Out:      outPath,
			Profiles: len(st.Profiles),
			Events:   len(bundle.Events),
			Roots:    len(bundle.Source.UsageRoots),
			Files:    len(bundle.Source.UsageFiles),
			Duration: time.Since(start),
		}}
	}
}

func exportTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return exportTickMsg{} })
}
