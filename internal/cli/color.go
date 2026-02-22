package cli

import (
	"os"
	"runtime"
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
