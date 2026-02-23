package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/derekurban/profilex-cli/internal/sync"
	"github.com/derekurban/profilex-cli/internal/usage"
)

func cmdSync(rootDir string, args []string) error {
	if len(args) == 0 || hasHelp(args) {
		printSyncHelp()
		return nil
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "init":
		return cmdSyncInit(rootDir, rest)
	case "status":
		return cmdSyncStatus(rootDir, rest)
	case "export":
		return cmdSyncExport(rootDir, rest)
	default:
		return fmt.Errorf("unknown sync subcommand %q", sub)
	}
}

func printSyncHelp() {
	fmt.Printf("Usage: profilex sync <command> [options]\n\n")
	fmt.Printf("Commands:\n")
	fmt.Printf("  init     Configure sync target (syncthing)\n")
	fmt.Printf("  status   Show sync config and target status\n")
	fmt.Printf("  export   Export unified usage bundle into sync target\n")
	fmt.Printf("\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  profilex sync init --provider syncthing --dir ~/Sync/profilex-usage --machine macbook\n")
	fmt.Printf("  profilex sync status\n")
	fmt.Printf("  profilex sync export --deep\n")
}

func cmdSyncInit(rootDir string, args []string) error {
	provider, args := extractFlag(args, "--provider")
	dir, args := extractFlag(args, "--dir")
	machine, args := extractFlag(args, "--machine")
	autoExport, args := extractBool(args, "--auto-export")

	if hasHelp(args) {
		fmt.Printf("Usage: profilex sync init --provider syncthing --dir <path> [--machine <name>] [--auto-export]\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %s", strings.Join(args, " "))
	}

	if strings.TrimSpace(provider) == "" {
		provider = "syncthing"
	}
	if strings.TrimSpace(machine) == "" {
		h, _ := os.Hostname()
		machine = h
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("--dir is required")
	}

	dir = expandSyncHome(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	cfg := &sync.Config{
		Provider:   provider,
		Directory:  dir,
		Machine:    machine,
		AutoExport: autoExport,
	}
	root, err := resolveRootDir(rootDir)
	if err != nil {
		return err
	}
	path := sync.ConfigPath(root)
	if err := sync.Save(path, cfg); err != nil {
		return err
	}

	fmt.Printf("%s Sync configured\n", Green("✓"))
	fmt.Printf("   Provider: %s\n", cfg.Provider)
	fmt.Printf("   Directory: %s\n", cfg.Directory)
	fmt.Printf("   Machine: %s\n", cfg.Machine)
	fmt.Printf("   Bundle: %s\n", filepath.Join(cfg.Directory, cfg.BundleFilename()))
	if cfg.Provider == "syncthing" {
		if _, err := exec.LookPath("syncthing"); err != nil {
			fmt.Printf("   %s syncthing binary not found in PATH (this is okay if Syncthing is already running as a service).\n", Yellow("⚠"))
		}
	}
	return nil
}

func cmdSyncStatus(rootDir string, args []string) error {
	if hasHelp(args) {
		fmt.Printf("Usage: profilex sync status\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %s", strings.Join(args, " "))
	}
	root, err := resolveRootDir(rootDir)
	if err != nil {
		return err
	}
	cfgPath := sync.ConfigPath(root)
	cfg, err := sync.Load(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No sync config found. Run: %s\n", Bold("profilex sync init --provider syncthing --dir <path>"))
			return nil
		}
		return err
	}

	bundlePath := filepath.Join(cfg.Directory, cfg.BundleFilename())
	_, stErr := os.Stat(bundlePath)

	fmt.Printf("%s\n\n", Bold("Sync status"))
	fmt.Printf("Config: %s\n", cfgPath)
	fmt.Printf("Provider: %s\n", cfg.Provider)
	fmt.Printf("Directory: %s\n", cfg.Directory)
	fmt.Printf("Machine: %s\n", cfg.Machine)
	fmt.Printf("Auto export: %v\n", cfg.AutoExport)
	fmt.Printf("Bundle path: %s\n", bundlePath)
	if stErr == nil {
		fmt.Printf("Bundle file: %s\n", Green("present"))
	} else {
		fmt.Printf("Bundle file: %s\n", Yellow("missing"))
	}
	if _, err := exec.LookPath("syncthing"); err == nil {
		fmt.Printf("Syncthing CLI: %s\n", Green("found"))
	} else {
		fmt.Printf("Syncthing CLI: %s\n", Dim("not found in PATH"))
	}
	return nil
}

func cmdSyncExport(rootDir string, args []string) error {
	deep, args := extractBool(args, "--deep")
	maxFilesRaw, args := extractFlag(args, "--max-files")
	tz, args := extractFlag(args, "--timezone")
	costModeRaw, args := extractFlag(args, "--cost-mode")
	outOverride, args := extractFlag(args, "--out")

	if hasHelp(args) {
		fmt.Printf("Usage: profilex sync export [--deep] [--max-files <n>] [--timezone <tz>] [--cost-mode <mode>] [--out <file>]\n")
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown argument(s): %s", strings.Join(args, " "))
	}

	root, err := resolveRootDir(rootDir)
	if err != nil {
		return err
	}
	cfg, err := sync.Load(sync.ConfigPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("sync is not configured; run profilex sync init first")
		}
		return err
	}

	outPath := strings.TrimSpace(outOverride)
	if outPath == "" {
		outPath = filepath.Join(cfg.Directory, cfg.BundleFilename())
	}
	outPath = expandSyncHome(outPath)

	maxFiles := 5000
	if strings.TrimSpace(maxFilesRaw) != "" {
		n, err := strconv.Atoi(maxFilesRaw)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid --max-files value: %q", maxFilesRaw)
		}
		maxFiles = n
	}
	if strings.TrimSpace(tz) == "" {
		tz = time.Now().Location().String()
	}
	costMode, err := parseCostMode(costModeRaw)
	if err != nil {
		return err
	}

	mgr, err := newManager(rootDir)
	if err != nil {
		return err
	}
	state, err := mgr.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	bundle, err := usage.GenerateBundle(ctx, state, filepath.Join(root, "state.json"), usage.GenerateOptions{
		RootDir:  root,
		Deep:     deep,
		MaxFiles: maxFiles,
		Timezone: tz,
		CostMode: costMode,
	})
	if err != nil {
		return err
	}
	if err := usage.WriteBundle(outPath, bundle); err != nil {
		return err
	}

	fmt.Printf("%s Synced usage bundle written\n", Green("✓"))
	fmt.Printf("   📄 File: %s\n", Dim(outPath))
	fmt.Printf("   🧾 Events: %d\n", len(bundle.Events))
	fmt.Printf("   📁 Roots: %d\n", len(bundle.Source.UsageRoots))
	fmt.Printf("   🗂️ Files: %d\n", len(bundle.Source.UsageFiles))
	return nil
}

func expandSyncHome(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if p == "~" {
		h, _ := os.UserHomeDir()
		return h
	}
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[2:])
	}
	return p
}
