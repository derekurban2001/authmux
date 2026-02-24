package cli

import (
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/derekurban/profilex-cli/internal/store"
)

var colorEnabled = detectColor()

func detectColor() bool {
	// Explicitly disabled
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if stdout is a terminal
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false // piped
	}

	// On Windows, modern terminals (Windows Terminal, VS Code) support ANSI.
	// Check for known indicators.
	if runtime.GOOS == "windows" {
		if os.Getenv("WT_SESSION") != "" {
			return true
		}
		if os.Getenv("TERM_PROGRAM") != "" {
			return true
		}
		// ConEmu, cmder, etc.
		if os.Getenv("ConEmuANSI") == "ON" {
			return true
		}
		// Default: assume modern Windows supports ANSI (Win10+)
		return true
	}

	return true
}

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
)

func colorize(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + reset
}

func Bold(s string) string   { return colorize(bold, s) }
func Dim(s string) string    { return colorize(dim, s) }
func Green(s string) string  { return colorize(green, s) }
func Yellow(s string) string { return colorize(yellow, s) }
func Red(s string) string    { return colorize(red, s) }
func Cyan(s string) string   { return colorize(cyan, s) }

// ---------------------------------------------------------------------------
// Lipgloss-based TUI design system
// ---------------------------------------------------------------------------

// Color palette
var (
	colorPrimary    = lipgloss.Color("#0F766E")
	colorAccent     = lipgloss.Color("#A7F3D0")
	colorSuccess    = lipgloss.Color("#065F46")
	colorError      = lipgloss.Color("#B91C1C")
	colorWarning    = lipgloss.Color("#92400E")
	colorMuted      = lipgloss.Color("#64748B")
	colorToggleOn   = lipgloss.Color("#065F46")
	colorToggleOff  = lipgloss.Color("#64748B")
	colorBadgeCl    = lipgloss.Color("#7C3AED") // claude purple
	colorBadgeCx    = lipgloss.Color("#0369A1") // codex blue
	colorHelpKey    = lipgloss.Color("#F8FAFC")
	colorHelpDesc   = lipgloss.Color("#94A3B8")
	colorCardBorder = lipgloss.Color("#64748B")
	colorHeaderFg   = lipgloss.Color("#F8FAFC")
	colorHeaderBg   = lipgloss.Color("#0F766E")
	colorWarningBg  = lipgloss.Color("#FEF3C7")
	colorWarningFg  = lipgloss.Color("#92400E")
)

// Reusable styles
var (
	styleHeader = lipgloss.NewStyle().
			Foreground(colorHeaderFg).
			Background(colorHeaderBg).
			Padding(0, 1).
			Bold(true)

	styleHelpBar = lipgloss.NewStyle().
			Foreground(colorHelpDesc)

	styleHelpKey = lipgloss.NewStyle().
			Foreground(colorHelpKey).
			Background(colorMuted).
			Padding(0, 1).
			Bold(true)

	styleHelpDesc = lipgloss.NewStyle().
			Foreground(colorHelpDesc)

	styleSectionTitle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	styleCard = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorCardBorder).
			Padding(1, 2)

	styleToggleOn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ECFDF5")).
			Background(colorToggleOn).
			Padding(0, 1).
			Bold(true)

	styleToggleOff = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(colorToggleOff).
			Padding(0, 1)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarningFg).
			Bold(true)

	styleBadgeClaude = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F8FAFC")).
				Background(colorBadgeCl).
				Padding(0, 1).
				Bold(true)

	styleBadgeCodex = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(colorBadgeCx).
			Padding(0, 1).
			Bold(true)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleOverlay = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorWarningFg).
			Padding(1, 2)
)

func renderToggle(on bool) string {
	if on {
		return styleToggleOn.Render("ON")
	}
	return styleToggleOff.Render("OFF")
}

func renderKeyHint(key, desc string) string {
	return styleHelpKey.Render(key) + " " + styleHelpDesc.Render(desc)
}

func renderToolBadge(tool store.Tool) string {
	if tool == store.ToolClaude {
		return styleBadgeClaude.Render("claude")
	}
	return styleBadgeCodex.Render("codex")
}

func renderDivider(width int) string {
	if width <= 0 {
		width = 36
	}
	return styleMuted.Render(strings.Repeat("\u2500", width))
}
