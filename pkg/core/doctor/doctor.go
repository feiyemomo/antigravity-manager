package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agy-tools/pkg/config"
	"agy-tools/pkg/core/credential"
	"agy-tools/pkg/core/process"
	"agy-tools/pkg/core/profile"
	"agy-tools/pkg/core/symlink"
	"agy-tools/pkg/types"
	"github.com/google/uuid"
)

// RunDiagnostics executes complete system health checks
func RunDiagnostics(autoFix bool) types.DoctorReport {
	var items []types.DiagnosticItem

	// 1. Process Safety Check
	procs, err := process.GetActiveProcesses()
	if err == nil && len(procs) > 0 {
		var lines []string
		for _, p := range procs {
			lines = append(lines, fmt.Sprintf("%s (PID: %d)", p.Name, p.PID))
		}
		items = append(items, types.DiagnosticItem{
			Category: "Process Safety",
			Name:     "Active Processes",
			Status:   "WARN",
			Message:  fmt.Sprintf("Antigravity process is currently running (%d active process(es))", len(procs)),
			Details:  strings.Join(lines, "\n"),
			Fix:      "Close Antigravity IDE and CLI before executing account switching.",
		})
	} else {
		items = append(items, types.DiagnosticItem{
			Category: "Process Safety",
			Name:     "Active Processes",
			Status:   "OK",
			Message:  "No conflicting Antigravity processes running.",
		})
	}

	// 2. Credential Storage Check
	readable, secret, credErr := credential.CheckReadable()
	if !readable || credErr != nil {
		items = append(items, types.DiagnosticItem{
			Category: "Credential Storage",
			Name:     "Windows Credential Manager",
			Status:   "WARN",
			Message:  fmt.Sprintf("Credential '%s' not found or unreadable", types.CredentialTarget),
			Details:  fmt.Sprintf("%v", credErr),
			Fix:      "Run 'agy-tools doctor --fix' or switch accounts to seed the credential.",
		})
	} else {
		var tokenObj struct {
			Token struct {
				Expiry      string `json:"expiry"`
				AccessToken string `json:"access_token"`
			} `json:"token"`
			IDToken string `json:"id_token"`
		}
		_ = json.Unmarshal([]byte(secret), &tokenObj)

		accountEmail := "Unknown"
		if tokenObj.IDToken != "" {
			parts := strings.Split(tokenObj.IDToken, ".")
			if len(parts) >= 2 {
				// decode basic payload
				accountEmail = parseJwtEmail(parts[1])
			}
		}

		isExpired := false
		var expTime time.Time
		if tokenObj.Token.Expiry != "" {
			if t, err := time.Parse(time.RFC3339, tokenObj.Token.Expiry); err == nil {
				expTime = t
				isExpired = time.Now().After(t)
			}
		}

		if isExpired {
			items = append(items, types.DiagnosticItem{
				Category: "Credential Storage",
				Name:     "Windows Credential Manager",
				Status:   "WARN",
				Message:  fmt.Sprintf("Credential '%s' is expired", types.CredentialTarget),
				Details:  fmt.Sprintf("Target: %s\nAccount: %s\nToken Expiry: %s [EXPIRED]", types.CredentialTarget, accountEmail, expTime.Local().Format("2006/01/02 15:04:05")),
				Fix:      "Run 'agy-tools accounts refresh' to renew expired tokens.",
			})
		} else {
			items = append(items, types.DiagnosticItem{
				Category: "Credential Storage",
				Name:     "Windows Credential Manager",
				Status:   "OK",
				Message:  fmt.Sprintf("Credential '%s' is valid (Account: %s)", types.CredentialTarget, accountEmail),
			})
		}
	}

	// 3. Profiles Storage Check
	profiles, _ := profile.ListProfiles()
	activeName := profile.GetActiveProfileName()

	if len(profiles) == 0 {
		items = append(items, types.DiagnosticItem{
			Category: "Profiles Storage",
			Name:     "Profiles Directory",
			Status:   "WARN",
			Message:  "No profiles found in storage directory",
			Details:  fmt.Sprintf("Directory: %s", config.ProfilesDir),
			Fix:      "Run 'agy-tools doctor --fix' to import current Antigravity credentials.",
		})
	} else {
		var profLines []string
		for _, p := range profiles {
			flag := ""
			if p.Name == activeName {
				flag = " [ACTIVE]"
			}
			profLines = append(profLines, fmt.Sprintf("- %s%s: %s", p.Name, flag, p.Email))
		}
		items = append(items, types.DiagnosticItem{
			Category: "Profiles Storage",
			Name:     "Profiles Directory",
			Status:   "OK",
			Message:  fmt.Sprintf("%d profile(s) loaded (Active: %s)", len(profiles), activeName),
			Details:  strings.Join(profLines, "\n"),
		})
	}

	// 4. Shared Symlinks Check
	var symlinkItems []types.DiagnosticItem
	if autoFix {
		symlinkItems = symlink.EnsureAllSharedLinks()
	} else {
		symlinkItems = symlink.VerifyAllSharedLinks()
	}
	items = append(items, symlinkItems...)

	// Auto-fix execution
	if autoFix {
		_ = config.EnsureDirs()

		// If no profiles, attempt auto-import
		if len(profiles) == 0 && activeName == "" {
			if data, err := os.ReadFile(config.PrimaryTokenFile); err == nil && len(data) > 0 {
				var meta types.ProfileMetadata
				meta.Name = "default"
				meta.CreatedAt = time.Now().UnixMilli()

				installID := ""
				if iData, err := os.ReadFile(config.PrimaryInstallationIDFile); err == nil {
					installID = strings.TrimSpace(string(iData))
				} else {
					installID = uuid.New().String()
				}

				settings := "{}"
				if sData, err := os.ReadFile(config.PrimarySettingsFile); err == nil {
					settings = string(sData)
				}

				_ = profile.SaveProfile("default", types.ProfilePayload{
					InstallationID:  installID,
					TokenPayload:    string(data),
					SettingsPayload: settings,
					Metadata:        meta,
				})
				_ = profile.SetActiveProfileName("default")
			}
		}
	}

	overallStatus := "OK"
	for _, item := range items {
		if item.Status == "FAIL" {
			overallStatus = "FAILED"
			break
		} else if item.Status == "WARN" && overallStatus != "FAILED" {
			overallStatus = "DEGRADED"
		}
	}

	return types.DoctorReport{
		Status:    overallStatus,
		Items:     items,
		Timestamp: time.Now(),
	}
}

// PrintReport outputs formatted diagnostic report to terminal
func PrintReport(report types.DoctorReport) {
	fmt.Println("\n=== Antigravity Multi-Account Diagnostic Report ===")
	fmt.Println()

	categories := make(map[string][]types.DiagnosticItem)
	for _, item := range report.Items {
		categories[item.Category] = append(categories[item.Category], item)
	}

	for cat, itemList := range categories {
		for _, item := range itemList {
			tag := "  OK  "
			if item.Status == "WARN" {
				tag = " WARN "
			} else if item.Status == "FAIL" {
				tag = " FAIL "
			}
			fmt.Printf("[%s] %s: %s\n", tag, cat, item.Message)
			if item.Details != "" {
				lines := strings.Split(item.Details, "\n")
				for _, l := range lines {
					fmt.Printf("        %s\n", l)
				}
			}
			if item.Fix != "" {
				fmt.Printf("        Fix: %s\n", item.Fix)
			}
		}
		fmt.Println()
	}

	fmt.Println("--- Summary ---")
	fmt.Printf("Overall Status: %s\n", report.Status)
	if report.Status != "OK" {
		fmt.Println("Tip: Run 'agy-tools doctor --fix' to automatically repair symlinks and default profiles.")
	}
	fmt.Println()
}

func parseJwtEmail(payloadBase64 string) string {
	// Base64URL decode
	s := payloadBase64
	if rem := len(s) % 4; rem != 0 {
		s += strings.Repeat("=", 4-rem)
	}
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	var decoded struct {
		Email string `json:"email"`
	}
	// Try unmarshal directly
	_ = json.Unmarshal([]byte(s), &decoded)
	return decoded.Email
}
