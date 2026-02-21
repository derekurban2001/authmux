package authmux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/derekurban2001/authmux/internal/app"
	"github.com/derekurban2001/authmux/internal/shim"
	"github.com/derekurban2001/authmux/internal/store"
	"github.com/derekurban2001/authmux/internal/tui"
)

func NewRootCmd() *cobra.Command {
	var rootDir string
	cmd := &cobra.Command{
		Use:           "authmux",
		Short:         "Multi-profile auth manager for Claude Code and Codex",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := managerFromRootFlag(rootDir)
			if err != nil {
				return err
			}
			return tui.Run(manager)
		},
	}
	cmd.PersistentFlags().StringVar(&rootDir, "root", "", "AuthMux state directory (default: $AUTHMUX_HOME or ~/.authmux)")

	cmd.AddCommand(newAddCmd(&rootDir))
	cmd.AddCommand(newListCmd(&rootDir))
	cmd.AddCommand(newUseCmd(&rootDir))
	cmd.AddCommand(newRunCmd(&rootDir))
	cmd.AddCommand(newStatusCmd(&rootDir))
	cmd.AddCommand(newLogoutCmd(&rootDir))
	cmd.AddCommand(newRenameCmd(&rootDir))
	cmd.AddCommand(newRemoveCmd(&rootDir))
	cmd.AddCommand(newShimCmd(&rootDir))
	cmd.AddCommand(newDoctorCmd(&rootDir))
	return cmd
}

func managerFromRootFlag(root string) (*app.Manager, error) {
	if strings.TrimSpace(root) == "" {
		return app.NewDefaultManager()
	}
	abs := root
	if !filepath.IsAbs(root) {
		cwd, _ := os.Getwd()
		abs = filepath.Join(cwd, root)
	}
	return app.NewManager(abs)
}

func parseTool(raw string) (store.Tool, error) {
	tool, ok := store.IsSupportedTool(raw)
	if !ok {
		return "", fmt.Errorf("unsupported tool %q (expected: claude or codex)", raw)
	}
	return tool, nil
}

func newAddCmd(root *string) *cobra.Command {
	return &cobra.Command{
		Use:   "add <tool> <profile>",
		Short: "Create a profile and launch the tool's login flow",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			profileName := args[1]
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			profile, created, err := manager.EnsureProfile(tool, profileName)
			if err != nil {
				return err
			}
			if created {
				fmt.Printf("Created profile %s/%s\n", tool, profileName)
			}
			if shimPath, err := installShimForProfile(profile); err != nil {
				fmt.Printf("Warning: could not install shim for %s/%s: %v\n", tool, profileName, err)
			} else {
				fmt.Printf("Installed shim: %s\n", shimPath)
			}
			fmt.Printf("Starting login for %s/%s...\n", tool, profileName)
			if err := manager.LoginProfile(context.Background(), profile); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			st, err := manager.StatusForProfile(ctx, profile)
			if err != nil {
				fmt.Printf("Login completed. Status check error: %v\n", err)
				return nil
			}
			fmt.Printf("Login completed. Logged in: %v\n", st.LoggedIn)
			return nil
		},
	}
}

func newListCmd(root *string) *cobra.Command {
	var toolFlag string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List profiles and their auth status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			st, err := manager.Load()
			if err != nil {
				return err
			}
			var filter *store.Tool
			if toolFlag != "" {
				tool, err := parseTool(toolFlag)
				if err != nil {
					return err
				}
				filter = &tool
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			rows, err := manager.StatusRows(ctx, filter)
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
				fmt.Println("No profiles found.")
				return nil
			}
			for _, r := range rows {
				marker := " "
				if st.Defaults[r.Profile.Tool] == r.Profile.Name {
					marker = "*"
				}
				status := "unknown"
				if r.Error != "" {
					status = "error: " + r.Error
				} else if r.Status.LoggedIn {
					status = "logged-in"
				} else {
					status = "logged-out"
				}
				fmt.Printf("%s %s/%s  %s\n", marker, r.Profile.Tool, r.Profile.Name, status)
			}
			fmt.Println("* = default")
			return nil
		},
	}
	cmd.Flags().StringVar(&toolFlag, "tool", "", "Filter by tool (claude|codex)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output JSON")
	return cmd
}

func newUseCmd(root *string) *cobra.Command {
	return &cobra.Command{
		Use:   "use <tool> <profile>",
		Short: "Set default profile for a tool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			if err := manager.SetDefault(tool, args[1]); err != nil {
				return err
			}
			fmt.Printf("Default for %s set to %s\n", tool, args[1])
			return nil
		},
	}
}

func newRunCmd(root *string) *cobra.Command {
	return &cobra.Command{
		Use:   "run <tool> [profile] -- [tool args...]",
		Short: "Run tool using the selected/default auth profile",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("missing tool argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dash := cmd.ArgsLenAtDash()
			pre := args
			toolArgs := []string{}
			if dash >= 0 {
				pre = args[:dash]
				toolArgs = args[dash:]
			}
			if len(pre) < 1 || len(pre) > 2 {
				return errors.New("usage: authmux run <tool> [profile] -- [tool args...]")
			}
			tool, err := parseTool(pre[0])
			if err != nil {
				return err
			}
			profileOptional := ""
			if len(pre) == 2 {
				profileOptional = pre[1]
			}
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			st, err := manager.Load()
			if err != nil {
				return err
			}
			profile, err := manager.ResolveProfile(st, tool, profileOptional)
			if err != nil {
				return err
			}
			if err := manager.RunTool(context.Background(), profile, toolArgs); err != nil {
				return err
			}
			return nil
		},
	}
}

func newStatusCmd(root *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status [tool] [profile]",
		Short: "Show auth status",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				st, err := manager.Load()
				if err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				rows, err := manager.StatusRows(ctx, nil)
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
					fmt.Println("No profiles found.")
					return nil
				}
				for _, r := range rows {
					marker := " "
					if st.Defaults[r.Profile.Tool] == r.Profile.Name {
						marker = "*"
					}
					state := "logged-out"
					if r.Status.LoggedIn {
						state = "logged-in"
					}
					if r.Error != "" {
						state = "error: " + r.Error
					}
					fmt.Printf("%s %s/%s  %s\n", marker, r.Profile.Tool, r.Profile.Name, state)
				}
				fmt.Println("* = default")
				return nil
			}
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			st, err := manager.Load()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				rows, err := manager.StatusRows(ctx, &tool)
				if err != nil {
					return err
				}
				if jsonOut {
					b, _ := json.MarshalIndent(rows, "", "  ")
					fmt.Println(string(b))
					return nil
				}
				for _, r := range rows {
					state := "logged-out"
					if r.Status.LoggedIn {
						state = "logged-in"
					}
					if r.Error != "" {
						state = "error: " + r.Error
					}
					marker := " "
					if st.Defaults[r.Profile.Tool] == r.Profile.Name {
						marker = "*"
					}
					fmt.Printf("%s %s/%s  %s\n", marker, r.Profile.Tool, r.Profile.Name, state)
				}
				return nil
			}
			profile, err := manager.GetProfile(st, tool, args[1])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			status, err := manager.StatusForProfile(ctx, profile)
			if err != nil {
				return err
			}
			if jsonOut {
				b, _ := json.MarshalIndent(map[string]any{"profile": profile, "status": status}, "", "  ")
				fmt.Println(string(b))
				return nil
			}
			fmt.Printf("%s/%s\n", profile.Tool, profile.Name)
			fmt.Printf("  dir: %s\n", profile.Dir)
			fmt.Printf("  logged in: %v\n", status.LoggedIn)
			if status.Method != "" {
				fmt.Printf("  method: %s\n", status.Method)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output JSON")
	return cmd
}

func newLogoutCmd(root *string) *cobra.Command {
	return &cobra.Command{
		Use:   "logout <tool> <profile>",
		Short: "Log out from one profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			st, err := manager.Load()
			if err != nil {
				return err
			}
			profile, err := manager.GetProfile(st, tool, args[1])
			if err != nil {
				return err
			}
			return manager.LogoutProfile(context.Background(), profile)
		},
	}
}

func newRenameCmd(root *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <tool> <old-profile> <new-profile>",
		Short: "Rename a profile",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			if err := manager.RenameProfile(tool, args[1], args[2]); err != nil {
				return err
			}
			fmt.Printf("Renamed %s/%s to %s\n", tool, args[1], args[2])
			return nil
		},
	}
}

func newRemoveCmd(root *string) *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "remove <tool> <profile>",
		Short: "Remove profile from registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			if err := manager.RemoveProfile(tool, args[1], purge); err != nil {
				return err
			}
			fmt.Printf("Removed %s/%s\n", tool, args[1])
			if purge {
				fmt.Println("Profile directory purged.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "Delete the profile directory from disk")
	return cmd
}

func newShimCmd(root *string) *cobra.Command {
	cmd := &cobra.Command{Use: "shim", Short: "Manage generated launcher shims"}
	cmd.AddCommand(newShimInstallCmd(root))
	cmd.AddCommand(newShimUninstallCmd(root))
	return cmd
}

func newShimInstallCmd(root *string) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate claude-<profile> and codex-<profile> commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(dir) == "" {
				d, err := shim.DefaultShimDir()
				if err != nil {
					return err
				}
				dir = d
			}
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			st, err := manager.Load()
			if err != nil {
				return err
			}
			bin, err := exec.LookPath("authmux")
			if err != nil {
				bin = resolveAuthmuxBin()
			}
			count := 0
			for _, p := range st.Profiles {
				if path, err := shim.Install(dir, p, bin); err == nil {
					fmt.Printf("installed: %s\n", path)
					count++
				}
			}
			fmt.Printf("Installed %d shim(s) in %s\n", count, dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory to install shims")
	return cmd
}

func installShimForProfile(profile store.Profile) (string, error) {
	dir, err := shim.DefaultShimDir()
	if err != nil {
		return "", err
	}
	return shim.Install(dir, profile, resolveAuthmuxBin())
}

func resolveAuthmuxBin() string {
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	if bin, err := exec.LookPath("authmux"); err == nil && strings.TrimSpace(bin) != "" {
		return bin
	}
	return "authmux"
}

func newShimUninstallCmd(root *string) *cobra.Command {
	var dir string
	var all bool
	cmd := &cobra.Command{
		Use:   "uninstall [tool] [profile]",
		Short: "Remove generated shims",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(dir) == "" {
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
				fmt.Printf("Removed %d shim(s)\n", len(removed))
				for _, p := range removed {
					fmt.Println("removed:", p)
				}
				return nil
			}
			if len(args) != 2 {
				return errors.New("provide <tool> <profile> or use --all")
			}
			tool, err := parseTool(args[0])
			if err != nil {
				return err
			}
			p := store.Profile{Tool: tool, Name: args[1]}
			if err := shim.Remove(dir, p); err != nil {
				return err
			}
			fmt.Printf("Removed shim %s\n", shim.Name(tool, args[1]))
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory containing shims")
	cmd.Flags().BoolVar(&all, "all", false, "Remove all authmux-generated shims in directory")
	return cmd
}

func newDoctorCmd(root *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check tool binaries, profile directories, and defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := managerFromRootFlag(*root)
			if err != nil {
				return err
			}
			report, err := manager.Doctor()
			if err != nil {
				return err
			}
			if jsonOut {
				b, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(b))
				return nil
			}
			fmt.Printf("Root: %s\n", report.RootDir)
			fmt.Printf("Profiles: %d\n", report.ProfilesTotal)
			for t, ok := range report.ToolBinaries {
				state := "missing"
				if ok {
					state = "ok"
				}
				fmt.Printf("Binary %-6s : %s\n", t, state)
			}
			if len(report.MissingDirs) > 0 {
				fmt.Println("Missing profile directories:")
				for _, v := range report.MissingDirs {
					fmt.Println("  -", v)
				}
			}
			if len(report.BadDefaults) > 0 {
				fmt.Println("Default profile issues:")
				for _, v := range report.BadDefaults {
					fmt.Println("  -", v)
				}
			}
			if len(report.MissingDirs) == 0 && len(report.BadDefaults) == 0 {
				fmt.Println("No structural issues found.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output JSON")
	return cmd
}

// end of file
