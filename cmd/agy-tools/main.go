package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"agy-tools/pkg/auth"
	"agy-tools/pkg/config"
	"agy-tools/pkg/core/doctor"
	"agy-tools/pkg/core/livesync"
	"agy-tools/pkg/core/profile"
	"agy-tools/pkg/core/switcher"
	"agy-tools/pkg/core/symlink"
	"agy-tools/pkg/server"
	"agy-tools/pkg/server/proxy"
	"agy-tools/pkg/store"
	"agy-tools/pkg/types"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	version = "2.0.0"

	// Flags
	flagPort       int
	flagHost       string
	flagNoOpen     bool
	flagFix        bool
	flagJSON       bool
	flagLive       bool
	flagForce      bool
	flagRestartLsp bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "agy-tools",
		Short: "Antigravity multi-account manager and API proxy (Native Go/Gin)",
		Long:  "agy-tools is a high-performance native utility for managing multiple Google Antigravity accounts with atomic credential switching, live IDE hot-sync, and usage quota visualization.",
		Run: func(cmd *cobra.Command, args []string) {
			// Default action when no subcommand is provided: start server
			runStart()
		},
	}

	// Global Flags
	rootCmd.PersistentFlags().BoolVar(&flagLive, "live", false, "Hot-sync live into Antigravity IDE without closing it")
	rootCmd.PersistentFlags().BoolVarP(&flagForce, "force", "f", false, "Force account switch, bypassing process safety checks")
	rootCmd.PersistentFlags().BoolVar(&flagRestartLsp, "restart-lsp", false, "Restart language_server background process")

	// ============================================
	// Subcommand: start
	// ============================================
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Web dashboard and proxy server",
		Run: func(cmd *cobra.Command, args []string) {
			runStart()
		},
	}
	startCmd.Flags().IntVarP(&flagPort, "port", "p", 38080, "Web server listening port")
	startCmd.Flags().StringVarP(&flagHost, "host", "H", "127.0.0.1", "Web server listening host")
	startCmd.Flags().BoolVar(&flagNoOpen, "no-open", false, "Do not open browser dashboard automatically")
	rootCmd.AddCommand(startCmd)

	// ============================================
	// Subcommand: doctor
	// ============================================
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics on credentials, profiles, symlinks, and process safety",
		Run: func(cmd *cobra.Command, args []string) {
			rep := doctor.RunDiagnostics(flagFix)
			if flagJSON {
				data, _ := json.MarshalIndent(rep, "", "  ")
				fmt.Println(string(data))
				return
			}
			doctor.PrintReport(rep)
		},
	}
	doctorCmd.Flags().BoolVar(&flagFix, "fix", false, "Automatically fix and repair detected configuration issues")
	doctorCmd.Flags().BoolVar(&flagJSON, "json", false, "Output report as JSON")
	rootCmd.AddCommand(doctorCmd)

	// ============================================
	// Subcommand: switch
	// ============================================
	switchCmd := &cobra.Command{
		Use:   "switch <account-or-profile>",
		Short: "Atomically switch Antigravity to the specified profile or account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target := args[0]
			skipSafety := flagLive || flagForce
			runSwitch(target, skipSafety, flagRestartLsp)
		},
	}
	rootCmd.AddCommand(switchCmd)

	// ============================================
	// Subcommand: rotate
	// ============================================
	rotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate Antigravity IDE to the next available account",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.LoadConfig()
			if !flagForce && cfg.Proxy.IdleDetectionEnabled {
				idleStatus, _ := livesync.CheckAntigravityIdle()
				if idleStatus.IsRunning && !idleStatus.IsIdle {
					fmt.Printf("[BLOCKED] Antigravity is currently busy (%s).\nPass --force to rotate anyway, or wait until current task completes.\n", idleStatus.Reason)
					os.Exit(1)
				}
			}

			acc, err := store.DefaultStore.RotateAntigravityAccount(true)
			if err != nil {
				fmt.Printf("[ERROR] Failed to rotate account: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[SUCCESS] Rotated Antigravity account to: %s (Tier: %s)\n", acc.Email, acc.Tier)
		},
	}
	rootCmd.AddCommand(rotateCmd)

	// ============================================
	// Subcommand: login (alias for accounts add)
	// ============================================
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with a Google account via OAuth 2.0",
		Run: func(cmd *cobra.Command, args []string) {
			runLogin()
		},
	}
	rootCmd.AddCommand(loginCmd)

	// ============================================
	// Subcommand: accounts
	// ============================================
	accountsCmd := &cobra.Command{
		Use:   "accounts",
		Short: "Manage Google accounts",
		Run: func(cmd *cobra.Command, args []string) {
			runAccountsList()
		},
	}
	accountsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all authenticated accounts",
		Run: func(cmd *cobra.Command, args []string) {
			runAccountsList()
		},
	}
	accountsAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new account via Google OAuth",
		Run: func(cmd *cobra.Command, args []string) {
			runLogin()
		},
	}
	accountsRefreshCmd := &cobra.Command{
		Use:   "refresh [id-or-email]",
		Short: "Refresh access tokens and quota for accounts",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				target := args[0]
				acc := store.DefaultStore.GetAccount(target)
				if acc == nil {
					fmt.Printf("[ERROR] Account not found: %s\n", target)
					os.Exit(1)
				}
				fmt.Printf("Refreshing account %s...\n", acc.Email)
				if err := store.DefaultStore.RefreshAccount(acc.ID); err != nil {
					fmt.Printf("[ERROR] Failed to refresh token: %v\n", err)
				}
				_ = store.DefaultStore.RefreshQuota(acc.ID)
				fmt.Printf("[SUCCESS] Account %s refreshed.\n", acc.Email)
			} else {
				fmt.Println("Refreshing all accounts...")
				store.DefaultStore.RefreshAllAccounts()
				fmt.Println("[SUCCESS] All accounts refreshed.")
			}
		},
	}
	accountsSwitchCmd := &cobra.Command{
		Use:   "switch <id-or-email>",
		Short: "Switch to specified account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			skipSafety := flagLive || flagForce
			runSwitch(args[0], skipSafety, flagRestartLsp)
		},
	}
	accountsRotateCmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate to next account",
		Run: func(cmd *cobra.Command, args []string) {
			acc, err := store.DefaultStore.RotateAntigravityAccount(true)
			if err != nil {
				fmt.Printf("[ERROR] Failed to rotate: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[SUCCESS] Rotated to: %s\n", acc.Email)
		},
	}
	accountsRemoveCmd := &cobra.Command{
		Use:   "remove <id-or-email>",
		Short: "Remove an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target := args[0]
			acc := store.DefaultStore.GetAccount(target)
			if acc == nil {
				fmt.Printf("[ERROR] Account not found: %s\n", target)
				os.Exit(1)
			}
			if store.DefaultStore.RemoveAccount(acc.ID) {
				fmt.Printf("[SUCCESS] Account %s removed.\n", acc.Email)
			} else {
				fmt.Printf("[ERROR] Failed to remove account %s\n", acc.Email)
			}
		},
	}

	accountsCmd.AddCommand(accountsListCmd, accountsAddCmd, accountsRefreshCmd, accountsSwitchCmd, accountsRotateCmd, accountsRemoveCmd)
	rootCmd.AddCommand(accountsCmd)

	// ============================================
	// Subcommand: profiles / profile
	// ============================================
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage isolated Antigravity profiles",
		Run: func(cmd *cobra.Command, args []string) {
			runProfilesList()
		},
	}
	profilesCmd := &cobra.Command{
		Use:   "profiles",
		Short: "List all profiles",
		Run: func(cmd *cobra.Command, args []string) {
			runProfilesList()
		},
	}
	profileListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		Run: func(cmd *cobra.Command, args []string) {
			runProfilesList()
		},
	}
	profileCurrentCmd := &cobra.Command{
		Use:   "current",
		Short: "Show currently active profile",
		Run: func(cmd *cobra.Command, args []string) {
			name := profile.GetActiveProfileName()
			if name == "" {
				name = "(none / unmanaged)"
			}
			fmt.Printf("Active Profile: %s\n", name)
		},
	}
	profileCreateCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new empty profile",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			err := profile.SaveProfile(name, types.ProfilePayload{
				InstallationID:  uuid.New().String(),
				TokenPayload:    "",
				SettingsPayload: "{}",
				Metadata: types.ProfileMetadata{
					Name:      name,
					CreatedAt: time.Now().UnixMilli(),
				},
			})
			if err != nil {
				fmt.Printf("[ERROR] Failed to create profile: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[SUCCESS] Profile '%s' created.\n", name)
		},
	}
	profileLinkCmd := &cobra.Command{
		Use:   "link",
		Short: "Ensure shared directory symlinks (conversations, skills) are established",
		Run: func(cmd *cobra.Command, args []string) {
			items := symlink.EnsureAllSharedLinks()
			hasFail := false
			for _, item := range items {
				fmt.Printf("[%s] %s: %s\n", item.Status, item.Name, item.Message)
				if item.Status == "FAIL" {
					hasFail = true
				}
			}
			if hasFail {
				fmt.Println("[WARN] Some shared directory symlinks could not be established.")
			} else {
				fmt.Println("[SUCCESS] All shared directory symlinks established.")
			}
		},
	}

	profileCmd.AddCommand(profileListCmd, profileCurrentCmd, profileCreateCmd, profileLinkCmd)
	rootCmd.AddCommand(profileCmd, profilesCmd)

	// ============================================
	// Subcommand: models
	// ============================================
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "List available models supported by the Antigravity backend",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available Models:")
			for _, m := range proxy.AvailableModels {
				fmt.Printf("  - %s\n", m)
			}
		},
	}
	rootCmd.AddCommand(modelsCmd)

	// ============================================
	// Subcommand: version
	// ============================================
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print agy-tools version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("agy-tools v%s (%s/%s, Go native)\n", version, runtime.GOOS, runtime.GOARCH)
		},
	}
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// -----------------------------------------------------------------------------
// Helper Execution Functions
// -----------------------------------------------------------------------------

func runStart() {
	if flagPort == 0 {
		flagPort = 38080
	}
	if flagHost == "" {
		flagHost = "127.0.0.1"
	}

	dashboardURL := fmt.Sprintf("http://%s:%d", flagHost, flagPort)

	// Single-instance check: if server is already running on this port, open browser and exit cleanly
	client := &http.Client{Timeout: 600 * time.Millisecond}
	if resp, err := client.Get(fmt.Sprintf("%s/api/antigravity/status", dashboardURL)); err == nil && resp != nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Println("[INFO] Antigravity Manager is already running. Opening dashboard in browser...")
			_ = openBrowser(dashboardURL)
			return
		}
	}

	fmt.Println("================================================================")
	fmt.Printf("       Google Antigravity Multi-Account Tool v%s\n", version)
	fmt.Println("================================================================")
	fmt.Printf(" Dashboard : %s\n", dashboardURL)
	fmt.Printf(" Proxy URL : %s/v1\n", dashboardURL)
	fmt.Println(" Status    : Ready & Serving")
	fmt.Println("================================================================")

	// Start background quota refresh
	store.DefaultStore.StartBackgroundQuotaFetch()

	if !flagNoOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(dashboardURL)
		}()
	}

	if err := server.StartServer(flagPort, flagHost); err != nil {
		fmt.Printf("[FATAL] Server terminated with error: %v\n", err)
		os.Exit(1)
	}
}

func runSwitch(target string, skipSafety bool, restartLsp bool) {
	cfg := config.LoadConfig()
	if !flagForce && cfg.Proxy.IdleDetectionEnabled {
		idleStatus, _ := livesync.CheckAntigravityIdle()
		if idleStatus.IsRunning && !idleStatus.IsIdle {
			fmt.Printf("[BLOCKED] Antigravity is currently busy (%s).\nPass --force to switch anyway, or wait until current task completes.\n", idleStatus.Reason)
			os.Exit(1)
		}
	}

	fmt.Printf("Attempting atomic switch to '%s' (skipSafety: %v)...\n", target, skipSafety)

	// 1. Try via TokenStore (resolves accounts, tokens, and profiles)
	acc, err := store.DefaultStore.SwitchAntigravityAccount(target, skipSafety)
	if err == nil && acc != nil {
		fmt.Printf("[SUCCESS] Successfully switched Antigravity account to: %s\n", acc.Email)
		fmt.Printf("  Account ID: %s\n", acc.ID)
		fmt.Printf("  Tier      : %s\n", acc.Tier)
		return
	}

	// 2. Fallback directly to profile switch
	res, pErr := switcher.SwitchProfile(target, switcher.SwitchOptions{
		SkipProcessCheck: skipSafety,
		RestartLsp:       restartLsp,
	})
	if pErr == nil && res != nil {
		fmt.Printf("[SUCCESS] Successfully switched Antigravity profile to: %s\n", res.ProfileName)
		return
	}

	// If failed, print friendly diagnostic error
	if err != nil {
		fmt.Printf("\n[SWITCH ERROR] %v\n\n", err)
		if strings.Contains(err.Error(), "PROCESS SAFETY VIOLATION") {
			fmt.Println("Tip: Use '--live' to hot-sync into the running Antigravity IDE without exiting.")
			fmt.Println("     Or run: agy-tools switch " + target + " --live")
		}
	} else if pErr != nil {
		fmt.Printf("\n[SWITCH ERROR] %v\n\n", pErr)
	}
	os.Exit(1)
}

func runLogin() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	authURL, resCh, errCh := auth.StartOAuthFlow(ctx)

	fmt.Println("================================================================")
	fmt.Println("             Google Account OAuth Login")
	fmt.Println("================================================================")
	fmt.Println("Opening your default browser to complete Google authorization...")
	fmt.Printf("If browser does not open automatically, visit:\n%s\n\n", authURL)
	fmt.Printf("Waiting for callback on http://127.0.0.1:%d/callback ...\n", types.OAuthRedirectPort)

	_ = openBrowser(authURL)

	select {
	case tokens := <-resCh:
		if tokens == nil {
			fmt.Println("[ERROR] Received empty tokens.")
			os.Exit(1)
		}

		email, name, err := auth.FetchUserInfo(tokens.AccessToken)
		if err != nil || email == "" {
			fmt.Printf("[ERROR] Failed to fetch Google user info: %v\n", err)
			os.Exit(1)
		}

		projectID, tier := auth.FetchProjectIDAndTier(tokens.AccessToken)
		if name == "" {
			name = strings.Split(email, "@")[0]
		}

		acc := store.DefaultStore.AddAccount(types.Account{
			Email:     email,
			Name:      name,
			Tokens:    *tokens,
			ProjectID: projectID,
			Tier:      tier,
		})

		_ = store.DefaultStore.RefreshQuota(acc.ID)

		fmt.Println("================================================================")
		fmt.Printf("🎉 Successfully logged in and registered account!\n")
		fmt.Printf("  Email     : %s\n", acc.Email)
		fmt.Printf("  Name      : %s\n", acc.Name)
		fmt.Printf("  Tier      : %s\n", acc.Tier)
		fmt.Printf("  Project ID: %s\n", acc.ProjectID)
		fmt.Println("================================================================")

	case err := <-errCh:
		fmt.Printf("[ERROR] Authorization failed: %v\n", err)
		os.Exit(1)
	case <-ctx.Done():
		fmt.Println("[ERROR] Login timed out (5 minutes exceeded).")
		os.Exit(1)
	}
}

func runAccountsList() {
	accounts := store.DefaultStore.GetAccounts()
	if len(accounts) == 0 {
		fmt.Println("No accounts registered yet. Run 'agy-tools login' to add a Google account.")
		return
	}

	activeAcc := store.DefaultStore.GetActiveAccount()
	activeID := ""
	if activeAcc != nil {
		activeID = activeAcc.ID
	}

	fmt.Println("\nAuthenticated Google Accounts:")
	fmt.Println(strings.Repeat("-", 80))
	for _, a := range accounts {
		status := " "
		if a.ID == activeID {
			status = "*"
		}

		quotaInfo := "No quota data"
		if a.Quota != nil {
			var parts []string
			for _, g := range a.Quota.Groups {
				var bStrs []string
				for _, b := range g.Buckets {
					bStrs = append(bStrs, fmt.Sprintf("%s: %.0f%% (reset %s)", b.DisplayName, b.RemainingFraction*100, b.ResetTime))
				}
				parts = append(parts, fmt.Sprintf("[%s: %s]", g.DisplayName, strings.Join(bStrs, ", ")))
			}
			if len(parts) > 0 {
				quotaInfo = strings.Join(parts, " ")
			}
		}

		fmt.Printf("[%s] %s (%s, Tier: %s)\n", status, a.Email, a.Name, a.Tier)
		fmt.Printf("    ID   : %s\n", a.ID)
		fmt.Printf("    Quota: %s\n", quotaInfo)
		fmt.Println()
	}
	fmt.Println("(*) indicates currently active Antigravity account")
	fmt.Println(strings.Repeat("-", 80))
}

func runProfilesList() {
	profiles, err := profile.ListProfiles()
	if err != nil {
		fmt.Printf("[ERROR] Failed to list profiles: %v\n", err)
		return
	}

	activeName := profile.GetActiveProfileName()

	fmt.Println("\nIsolated Antigravity Profiles (~/.agy_auth/profiles):")
	fmt.Println(strings.Repeat("-", 60))
	for _, p := range profiles {
		status := " "
		if p.Name == activeName {
			status = "*"
		}
		fmt.Printf("[%s] %s (Email: %s, Tier: %s)\n", status, p.Name, p.Email, p.Tier)
	}
	if len(profiles) == 0 {
		fmt.Println("  (No profiles created yet)")
	}
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("(*) indicates currently active profile")
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "start", "", url)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err == nil {
			return nil
		}
		cmd2 := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		cmd2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd2.Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
