package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/derekurban/proflex-cli/internal/app"
	"github.com/derekurban/proflex-cli/internal/shim"
	"github.com/derekurban/proflex-cli/internal/store"
)

// Run parses os.Args[1:] and dispatches to the appropriate command.
// Returns the process exit code.
func Run(args []string) int {
	rootDir, args := extractFlag(args, "--root")

	if len(args) == 0 {
		printHelp()
		return 0
	}

	cmd := args[0]
	rest := args[1:]

	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printHelp()
		return 0
	}
	if cmd == "--version" || cmd == "version" {
		fmt.Println("proflex dev")
		return 0
	}

	var err error
	switch cmd {
	case "add":
		err = cmdAdd(rootDir, rest)
	case "remove":
		err = cmdRemove(rootDir, rest)
	case "uninstall":
		err = cmdUninstall(rootDir, rest)
	case "list":
		err = cmdList(rootDir, rest)
	case "run":
		err = cmdRun(rootDir, rest)
	case "use":
		err = cmdUse(rootDir, rest)
	case "rename":
		err = cmdRename(rootDir, rest)
	case "shim":
		err = cmdShim(rootDir, rest)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printHelp()
		return 1
	}

	if err != nil {
		var codeErr app.ExitCodeError
		if errors.As(err, &codeErr) {
			return codeErr.Code
		}
		fmt.Fprintf(os.Stderr, "%s %s\n", Red("error:"), err)
		return 1
	}
	return 0
}

func printHelp() {
	fmt.Printf(`%s - profile manager for Claude Code & Codex CLI

%s
  proflex <command> [options]

%s
  add <tool> <profile>          Create a new profile and install its shim
  remove <tool> <profile>       Remove a profile and its shim
  uninstall [--purge]           Uninstall proflex from this machine
  list [--tool <t>] [--json]    List all profiles with auth status
  use <tool> <profile>          Set the default profile for a tool
  rename <tool> <old> <new>     Rename a profile
  run <tool> [profile] -- ...   Run a tool with the given profile
  shim install [--dir <d>]      Reinstall shims for all profiles
  shim uninstall [--all]        Remove shims

%s
  --root <dir>     Override state directory (default: ~/.proflex)

%s
  proflex add claude work
  proflex add codex personal
  proflex list
  proflex use claude work
  claude-work                   %s
`,
		Bold("proflex"),
		Bold("Usage:"),
		Bold("Commands:"),
		Bold("Global options:"),
		Bold("Examples:"),
		Dim("(runs Claude Code with the 'work' profile)"),
	)
}

// --- add ---

func cmdAdd(rootDir string, args []string) error {
	if hasHelp(args) || len(args) < 2 {
		fmt.Printf("Usage: proflex add <tool> <profile>\n\n")
		fmt.Printf("Supported tools: %s\n", strings.Join(toolNames(), ", "))
		return nil
	}

	tool, err := parseTool(args[0])
	if err != nil {
		return err
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	profile, created, err := mgr.EnsureProfile(tool, args[1])
	if err != nil {
		return err
	}

	if !created {
		fmt.Printf("%s Profile %s already exists\n", Yellow("!"), Bold(string(tool)+"/"+profile.Name))
		return nil
	}

	shimPath, shimErr := installShimForProfile(profile)

	fmt.Printf("%s Created profile %s\n", Green("✓"), Bold(string(tool)+"/"+profile.Name))
	fmt.Printf("   📁 Config: %s\n", Dim(profile.Dir))

	if shimErr != nil {
		fmt.Printf("   %s Could not install shim: %v\n", Yellow("⚠"), shimErr)
	} else {
		fmt.Printf("   🔗 Shim:   %s\n", Cyan(shimPath))
	}

	shimName := shim.Name(tool, profile.Name)
	fmt.Println()
	fmt.Printf("   💡 Run %s to launch %s with this profile.\n", Bold(shimName), tool)
	fmt.Printf("      On first run you'll be prompted to authenticate.\n")

	return nil
}

// --- remove ---

func cmdRemove(rootDir string, args []string) error {
	purge, args := extractBool(args, "--purge")

	if hasHelp(args) || len(args) < 2 {
		fmt.Printf("Usage: proflex remove <tool> <profile> [--purge]\n")
		return nil
	}

	tool, err := parseTool(args[0])
	if err != nil {
		return err
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	// Remove shim first (best-effort)
	shimDir, _ := shim.DefaultShimDir()
	if shimDir != "" {
		_ = shim.Remove(shimDir, store.Profile{Tool: tool, Name: args[1]})
	}

	if err := mgr.RemoveProfile(tool, args[1], purge); err != nil {
		return err
	}

	shimName := shim.Name(tool, args[1])
	fmt.Printf("%s Removed profile %s\n", Green("✓"), Bold(string(tool)+"/"+args[1]))
	fmt.Printf("   Shim %s has been uninstalled.\n", Cyan(shimName))
	if purge {
		fmt.Printf("   Profile directory purged from disk.\n")
	}

	return nil
}

// --- uninstall ---

func cmdUninstall(rootDir string, args []string) error {
	purge, args := extractBool(args, "--purge")

	if hasHelp(args) {
		fmt.Printf("Usage: proflex uninstall [--purge]\n\n")
		fmt.Printf("  --purge  Remove profile state directory (~/.proflex)\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %s", strings.Join(args, " "))
	}

	var summary []string

	shimDir, err := shim.DefaultShimDir()
	if err != nil {
		return err
	}
	removed, err := shim.RemoveAll(shimDir)
	if err != nil {
		return err
	}
	summary = append(summary, fmt.Sprintf("Removed %d proflex shim(s) from %s", len(removed), shimDir))

	if purge {
		stateRoot, err := resolveRootDir(rootDir)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(stateRoot); err != nil {
			return fmt.Errorf("remove state dir %s: %w", stateRoot, err)
		}
		summary = append(summary, fmt.Sprintf("Removed state directory %s", stateRoot))
	}

	binCandidates := proflexBinaryCandidates()
	binRemoved := []string{}
	for _, candidate := range binCandidates {
		removed, err := removeFileWithWindowsFallback(candidate)
		if err != nil {
			return fmt.Errorf("remove binary %s: %w", candidate, err)
		}
		if removed {
			binRemoved = append(binRemoved, candidate)
		}
	}
	if len(binRemoved) == 0 {
		summary = append(summary, "Could not remove proflex binary automatically")
	} else {
		for _, path := range binRemoved {
			summary = append(summary, "Removed binary "+path)
		}
	}

	fmt.Printf("%s Uninstall complete\n", Green("✓"))
	for _, line := range summary {
		fmt.Printf("   - %s\n", line)
	}
	if len(binRemoved) == 0 {
		fmt.Printf("   - %s\n", "If proflex is still on PATH, remove it manually from your install directory.")
	}
	return nil
}

// --- list ---

func cmdList(rootDir string, args []string) error {
	toolFlag, args := extractFlag(args, "--tool")
	jsonOut, _ := extractBool(args, "--json")

	if hasHelp(args) {
		fmt.Printf("Usage: proflex list [--tool <tool>] [--json]\n")
		return nil
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	st, err := mgr.Load()
	if err != nil {
		return err
	}

	var filter *store.Tool
	if toolFlag != "" {
		t, e := parseTool(toolFlag)
		if e != nil {
			return e
		}
		filter = &t
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rows, err := mgr.StatusRows(ctx, filter)
	if err != nil {
		return err
	}

	if jsonOut {
		payload := map[string]any{"defaults": st.Defaults, "profiles": rows}
		b, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	if len(rows) == 0 {
		fmt.Printf("No profiles found.\n\n")
		fmt.Printf("💡 Get started: %s\n", Bold("proflex add claude <profile-name>"))
		return nil
	}

	fmt.Printf("%s\n\n", Bold("📋 Profiles"))

	// Group by tool
	var currentTool store.Tool
	var hints []string
	for _, r := range rows {
		if r.Profile.Tool != currentTool {
			if currentTool != "" {
				fmt.Println()
			}
			currentTool = r.Profile.Tool
			fmt.Printf("  %s\n", Bold(string(currentTool)))
		}

		isDefault := st.Defaults[r.Profile.Tool] == r.Profile.Name
		var icon, status, suffix string

		if r.Error != "" {
			icon = Yellow("⚠")
			status = Dim("error")
		} else if r.Status.LoggedIn {
			icon = Green("●")
			status = Green("logged in")
		} else {
			icon = Dim("○")
			status = Dim("not authenticated")
		}

		if isDefault {
			suffix = " " + Cyan("(default)")
		}

		shimName := shim.Name(r.Profile.Tool, r.Profile.Name)
		fmt.Printf("    %s %-20s %s%s\n", icon, r.Profile.Name, status, suffix)
		hints = append(hints, shimName)
	}

	fmt.Println()
	if len(hints) > 0 {
		fmt.Printf("💡 Run %s to launch with that profile.\n", Bold(hints[0]))
	}

	return nil
}

// --- use ---

func cmdUse(rootDir string, args []string) error {
	if hasHelp(args) || len(args) < 2 {
		fmt.Printf("Usage: proflex use <tool> <profile>\n")
		return nil
	}

	tool, err := parseTool(args[0])
	if err != nil {
		return err
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	if err := mgr.SetDefault(tool, args[1]); err != nil {
		return err
	}

	fmt.Printf("%s Default for %s set to %s\n", Green("✓"), Bold(string(tool)), Bold(args[1]))
	return nil
}

// --- rename ---

func cmdRename(rootDir string, args []string) error {
	if hasHelp(args) || len(args) < 3 {
		fmt.Printf("Usage: proflex rename <tool> <old-name> <new-name>\n")
		return nil
	}

	tool, err := parseTool(args[0])
	if err != nil {
		return err
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	oldName, newName := args[1], args[2]

	// Remove old shim, install new one
	shimDir, _ := shim.DefaultShimDir()
	if shimDir != "" {
		_ = shim.Remove(shimDir, store.Profile{Tool: tool, Name: oldName})
	}

	if err := mgr.RenameProfile(tool, oldName, newName); err != nil {
		return err
	}

	// Install new shim
	st, _ := mgr.Load()
	if st != nil {
		if _, p := store.FindProfile(st, tool, newName); p != nil {
			if path, err := installShimForProfile(*p); err == nil {
				fmt.Printf("   🔗 Shim updated: %s\n", Cyan(path))
			}
		}
	}

	fmt.Printf("%s Renamed %s to %s\n", Green("✓"),
		Bold(string(tool)+"/"+oldName),
		Bold(string(tool)+"/"+newName))

	return nil
}

// --- run ---

func cmdRun(rootDir string, args []string) error {
	if hasHelp(args) || len(args) < 1 {
		fmt.Printf("Usage: proflex run <tool> [profile] -- [tool args...]\n")
		return nil
	}

	// Split on "--"
	pre, toolArgs := splitDash(args)

	if len(pre) < 1 || len(pre) > 2 {
		return fmt.Errorf("usage: proflex run <tool> [profile] -- [tool args...]")
	}

	tool, err := parseTool(pre[0])
	if err != nil {
		return err
	}

	profileOptional := ""
	if len(pre) == 2 {
		profileOptional = pre[1]
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	st, err := mgr.Load()
	if err != nil {
		return err
	}

	profile, err := mgr.ResolveProfile(st, tool, profileOptional)
	if err != nil {
		return err
	}

	return mgr.RunTool(context.Background(), profile, toolArgs)
}

// --- shim ---

func cmdShim(rootDir string, args []string) error {
	if len(args) == 0 || hasHelp(args) {
		fmt.Printf("Usage:\n")
		fmt.Printf("  proflex shim install [--dir <d>]\n")
		fmt.Printf("  proflex shim uninstall [--all] [<tool> <profile>]\n")
		return nil
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "install":
		return cmdShimInstall(rootDir, rest)
	case "uninstall":
		return cmdShimUninstall(rootDir, rest)
	default:
		return fmt.Errorf("unknown shim subcommand: %s", sub)
	}
}

func cmdShimInstall(rootDir string, args []string) error {
	dir, _ := extractFlag(args, "--dir")
	if dir == "" {
		d, err := shim.DefaultShimDir()
		if err != nil {
			return err
		}
		dir = d
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}

	st, err := mgr.Load()
	if err != nil {
		return err
	}

	bin := resolveProflexBin()
	count := 0
	for _, p := range st.Profiles {
		if path, err := shim.Install(dir, p, bin); err == nil {
			fmt.Printf("   🔗 %s\n", Cyan(path))
			count++
		}
	}

	fmt.Printf("\n%s Installed %d shim(s) in %s\n", Green("✓"), count, Dim(dir))
	return nil
}

func cmdShimUninstall(rootDir string, args []string) error {
	all, args := extractBool(args, "--all")
	dir, args := extractFlag(args, "--dir")

	if dir == "" {
		d, err := shim.DefaultShimDir()
		if err != nil {
			return err
		}
		dir = d
	}

	if all {
		removed, err := shim.RemoveAll(dir)
		if err != nil {
			return err
		}
		for _, p := range removed {
			fmt.Printf("   removed: %s\n", p)
		}
		fmt.Printf("%s Removed %d shim(s)\n", Green("✓"), len(removed))
		return nil
	}

	if len(args) != 2 {
		return fmt.Errorf("provide <tool> <profile> or use --all")
	}

	tool, err := parseTool(args[0])
	if err != nil {
		return err
	}

	p := store.Profile{Tool: tool, Name: args[1]}
	if err := shim.Remove(dir, p); err != nil {
		return err
	}

	fmt.Printf("%s Removed shim %s\n", Green("✓"), Cyan(shim.Name(tool, args[1])))
	return nil
}

// --- helpers ---

func newManager(rootDir string) (*app.Manager, error) {
	if strings.TrimSpace(rootDir) == "" {
		return app.NewDefaultManager()
	}
	abs := rootDir
	if !filepath.IsAbs(rootDir) {
		cwd, _ := os.Getwd()
		abs = filepath.Join(cwd, rootDir)
	}
	return app.NewManager(abs)
}

func parseTool(raw string) (store.Tool, error) {
	tool, ok := store.IsSupportedTool(raw)
	if !ok {
		return "", fmt.Errorf("unsupported tool %q (supported: %s)", raw, strings.Join(toolNames(), ", "))
	}
	return tool, nil
}

func toolNames() []string {
	names := make([]string, len(store.SupportedTools))
	for i, t := range store.SupportedTools {
		names[i] = string(t)
	}
	return names
}

func installShimForProfile(profile store.Profile) (string, error) {
	dir, err := shim.DefaultShimDir()
	if err != nil {
		return "", err
	}
	return shim.Install(dir, profile, resolveProflexBin())
}

func resolveProflexBin() string {
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	if bin, err := exec.LookPath("proflex"); err == nil && strings.TrimSpace(bin) != "" {
		return bin
	}
	return "proflex"
}

func resolveRootDir(rootDir string) (string, error) {
	if strings.TrimSpace(rootDir) == "" {
		return store.DefaultRoot()
	}
	abs := rootDir
	if !filepath.IsAbs(rootDir) {
		cwd, _ := os.Getwd()
		abs = filepath.Join(cwd, rootDir)
	}
	return abs, nil
}

func proflexBinaryCandidates() []string {
	var candidates []string

	if exe, err := os.Executable(); err == nil && isProflexBinaryPath(exe) {
		candidates = append(candidates, exe)
	}
	if lp, err := exec.LookPath("proflex"); err == nil && isProflexBinaryPath(lp) {
		candidates = append(candidates, lp)
	}
	if lp, err := exec.LookPath("proflex.exe"); err == nil && isProflexBinaryPath(lp) {
		candidates = append(candidates, lp)
	}

	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		installDir := os.Getenv("PROFLEX_INSTALL_DIR")
		if strings.TrimSpace(installDir) == "" {
			installDir = filepath.Join(home, ".local", "bin")
		}
		candidates = append(candidates,
			filepath.Join(installDir, "proflex"),
			filepath.Join(installDir, "proflex.exe"),
		)
	}

	return uniquePaths(candidates)
}

func isProflexBinaryPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return base == "proflex" || base == "proflex.exe"
}

func uniquePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		key := filepath.Clean(p)
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

func removeFileWithWindowsFallback(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}

	if err := os.Remove(path); err == nil {
		return true, nil
	} else if runtime.GOOS != "windows" {
		return false, err
	}

	escaped := strings.ReplaceAll(path, `"`, `""`)
	cmd := exec.Command("cmd", "/C", fmt.Sprintf(`ping 127.0.0.1 -n 2 >nul & del /f /q "%s"`, escaped))
	if err := cmd.Start(); err != nil {
		return false, err
	}
	_ = cmd.Process.Release()
	return true, nil
}

// extractFlag extracts "--flag value" from args, returning the value and remaining args.
func extractFlag(args []string, flag string) (string, []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			val := args[i+1]
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return val, remaining
		}
	}
	return "", args
}

// extractBool extracts a boolean "--flag" from args.
func extractBool(args []string, flag string) (bool, []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}

// splitDash splits args on "--", returning pre and post.
func splitDash(args []string) ([]string, []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func hasHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}
