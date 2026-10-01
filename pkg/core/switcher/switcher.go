package switcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agy-tools/pkg/config"
	"agy-tools/pkg/core/credential"
	"agy-tools/pkg/core/livesync"
	"agy-tools/pkg/core/process"
	"agy-tools/pkg/core/profile"
	"agy-tools/pkg/core/symlink"
	"agy-tools/pkg/types"
	"github.com/google/uuid"
)

type SwitchOptions struct {
	SkipProcessCheck bool
	RestartLsp       bool
	QuotaGroups      []types.QuotaGroup
}

type SwitchResult struct {
	Success             bool   `json:"success"`
	ProfileName         string `json:"profileName"`
	Email               string `json:"email,omitempty"`
	PreviousProfileName string `json:"previousProfileName,omitempty"`
}

// AtomicWriteFile writes data to a temporary file in the same directory and renames it
func AtomicWriteFile(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	_ = os.MkdirAll(dir, 0755)

	tempFile := filepath.Join(dir, fmt.Sprintf(".tmp_%s_%s", filepath.Base(targetPath), uuid.New().String()[:8]))
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	// On Windows, os.Rename fails if target exists in older Go versions unless removed first
	_ = os.Remove(targetPath)
	if err := os.Rename(tempFile, targetPath); err != nil {
		_ = os.Remove(tempFile)
		return os.WriteFile(targetPath, data, 0644)
	}

	return nil
}

// SyncCurrentStateToActiveProfile captures current on-disk tokens & files back into the active profile folder
func SyncCurrentStateToActiveProfile() error {
	activeName := profile.GetActiveProfileName()
	if activeName == "" {
		return nil
	}

	p, err := profile.GetProfile(activeName)
	if err != nil || p == nil {
		return nil
	}

	installationID := p.InstallationID
	if data, err := os.ReadFile(config.PrimaryInstallationIDFile); err == nil {
		installationID = strings.TrimSpace(string(data))
	}

	tokenPayload := p.TokenPayload
	if credSecret, err := credential.Read(types.CredentialTarget); err == nil && credSecret != "" {
		tokenPayload = credSecret
	} else if data, err := os.ReadFile(config.PrimaryTokenFile); err == nil {
		tokenPayload = string(data)
	}

	settingsPayload := p.SettingsPayload
	if data, err := os.ReadFile(config.PrimarySettingsFile); err == nil {
		settingsPayload = string(data)
	}

	return profile.SaveProfile(activeName, types.ProfilePayload{
		InstallationID:  installationID,
		TokenPayload:    tokenPayload,
		SettingsPayload: settingsPayload,
		Metadata: types.ProfileMetadata{
			Name:       activeName,
			Email:      p.Metadata.Email,
			AccountID:  p.Metadata.AccountID,
			Tier:       p.Metadata.Tier,
			LastUsedAt: time.Now().UnixMilli(),
			CreatedAt:  p.Metadata.CreatedAt,
		},
	})
}

// SwitchProfile atomically switches to the target profile
func SwitchProfile(targetName string, opts SwitchOptions) (*SwitchResult, error) {
	// 1. Resolve target profile
	targetPayload, resolvedName := profile.ResolveProfile(targetName)
	if targetPayload == nil {
		return nil, fmt.Errorf("profile '%s' not found", targetName)
	}
	if resolvedName != "" {
		targetName = resolvedName
	}
	// 0. Snapshot active conversation path if Antigravity is running
	savedConvoPath := livesync.GetCurrentConversationPath()

	previousActiveName := profile.GetActiveProfileName()

	// 2. Process Safety Check
	if !opts.SkipProcessCheck {
		if err := process.CheckProcessSafety(false); err != nil {
			return nil, err
		}
	}

	// 3. Sync current active state back before switching
	if previousActiveName != "" && previousActiveName != targetName {
		_ = SyncCurrentStateToActiveProfile()
	}

	// 4. Capture current state for rollback
	rollbackSnapshots := make(map[string]*string)
	filesToTrack := []string{
		config.PrimaryInstallationIDFile,
		config.IDEInstallationIDFile,
		config.AntigravityDirInstallIDFile,
		config.AntigravityStateFile,
		config.PrimaryTokenFile,
		config.SecondaryTokenFile,
		config.AntigravityDirTokenFile,
		config.AntigravityDirAltTokenFile,
		config.PrimarySettingsFile,
		config.AntigravityDirSettingsFile,
	}

	for _, f := range filesToTrack {
		if data, err := os.ReadFile(f); err == nil {
			s := string(data)
			rollbackSnapshots[f] = &s
		} else {
			rollbackSnapshots[f] = nil
		}
	}

	var previousCredentialSecret *string
	if sec, err := credential.Read(types.CredentialTarget); err == nil {
		previousCredentialSecret = &sec
	}

	var rollbackActions []func()

	// Helper to register rollback
	pushRollback := func(fn func()) {
		rollbackActions = append(rollbackActions, fn)
	}

	// Execute switch with rollback guard
	execute := func() error {
		// A. Installation IDs
		installID := []byte(strings.TrimSpace(targetPayload.InstallationID))
		if err := AtomicWriteFile(config.PrimaryInstallationIDFile, installID); err != nil {
			return err
		}
		pushRollback(func() {
			if orig := rollbackSnapshots[config.PrimaryInstallationIDFile]; orig != nil {
				_ = os.WriteFile(config.PrimaryInstallationIDFile, []byte(*orig), 0644)
			} else {
				_ = os.Remove(config.PrimaryInstallationIDFile)
			}
		})

		if _, err := os.Stat(config.AntigravityIDEDir); err == nil {
			_ = AtomicWriteFile(config.IDEInstallationIDFile, installID)
			pushRollback(func() {
				if orig := rollbackSnapshots[config.IDEInstallationIDFile]; orig != nil {
					_ = os.WriteFile(config.IDEInstallationIDFile, []byte(*orig), 0644)
				}
			})
		}

		if _, err := os.Stat(filepath.Dir(config.AntigravityDirInstallIDFile)); err == nil {
			_ = AtomicWriteFile(config.AntigravityDirInstallIDFile, installID)
			pushRollback(func() {
				if orig := rollbackSnapshots[config.AntigravityDirInstallIDFile]; orig != nil {
					_ = os.WriteFile(config.AntigravityDirInstallIDFile, []byte(*orig), 0644)
				}
			})
		}

		// Update antigravity_state.pbtxt if present
		if stateData, err := os.ReadFile(config.AntigravityStateFile); err == nil {
			stateContent := string(stateData)
			lines := strings.Split(stateContent, "\n")
			for i, l := range lines {
				if strings.HasPrefix(strings.TrimSpace(l), "installation_uuid:") {
					lines[i] = fmt.Sprintf("installation_uuid: %q", string(installID))
				}
			}
			_ = os.WriteFile(config.AntigravityStateFile, []byte(strings.Join(lines, "\n")), 0644)
			pushRollback(func() {
				if orig := rollbackSnapshots[config.AntigravityStateFile]; orig != nil {
					_ = os.WriteFile(config.AntigravityStateFile, []byte(*orig), 0644)
				}
			})
		}

		// B. Token Files
		tokenData := []byte(targetPayload.TokenPayload)
		_ = AtomicWriteFile(config.PrimaryTokenFile, tokenData)
		pushRollback(func() {
			if orig := rollbackSnapshots[config.PrimaryTokenFile]; orig != nil {
				_ = os.WriteFile(config.PrimaryTokenFile, []byte(*orig), 0644)
			}
		})

		_ = AtomicWriteFile(config.SecondaryTokenFile, tokenData)
		pushRollback(func() {
			if orig := rollbackSnapshots[config.SecondaryTokenFile]; orig != nil {
				_ = os.WriteFile(config.SecondaryTokenFile, []byte(*orig), 0644)
			}
		})

		_ = AtomicWriteFile(config.AntigravityDirTokenFile, tokenData)
		pushRollback(func() {
			if orig := rollbackSnapshots[config.AntigravityDirTokenFile]; orig != nil {
				_ = os.WriteFile(config.AntigravityDirTokenFile, []byte(*orig), 0644)
			}
		})

		_ = AtomicWriteFile(config.AntigravityDirAltTokenFile, tokenData)
		pushRollback(func() {
			if orig := rollbackSnapshots[config.AntigravityDirAltTokenFile]; orig != nil {
				_ = os.WriteFile(config.AntigravityDirAltTokenFile, []byte(*orig), 0644)
			}
		})

		// C. Settings Files
		if targetPayload.SettingsPayload != "" {
			settingsData := []byte(targetPayload.SettingsPayload)
			_ = AtomicWriteFile(config.PrimarySettingsFile, settingsData)
			pushRollback(func() {
				if orig := rollbackSnapshots[config.PrimarySettingsFile]; orig != nil {
					_ = os.WriteFile(config.PrimarySettingsFile, []byte(*orig), 0644)
				}
			})

			_ = AtomicWriteFile(config.AntigravityDirSettingsFile, settingsData)
			pushRollback(func() {
				if orig := rollbackSnapshots[config.AntigravityDirSettingsFile]; orig != nil {
					_ = os.WriteFile(config.AntigravityDirSettingsFile, []byte(*orig), 0644)
				}
			})
		}

		// D. Windows Credential Manager
		if err := credential.Write(types.CredentialTarget, targetPayload.TokenPayload); err != nil {
			return fmt.Errorf("failed to write Windows Credential Manager: %w", err)
		}
		pushRollback(func() {
			if previousCredentialSecret != nil {
				_ = credential.Write(types.CredentialTarget, *previousCredentialSecret)
			} else {
				_ = credential.Delete(types.CredentialTarget)
			}
		})

		// E. Active Marker
		if err := profile.SetActiveProfileName(targetName); err != nil {
			return err
		}
		pushRollback(func() {
			if previousActiveName != "" {
				_ = profile.SetActiveProfileName(previousActiveName)
			}
		})

		// F. Shared symlinks health
		_ = symlink.EnsureAllSharedLinks()

		// G. Live UI Sync for running Antigravity IDE
		if targetPayload.Metadata.Email != "" {
			var accessToken string
			var parsed struct {
				Token struct {
					AccessToken string `json:"access_token"`
				} `json:"token"`
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal([]byte(targetPayload.TokenPayload), &parsed); err == nil {
				if parsed.Token.AccessToken != "" {
					accessToken = parsed.Token.AccessToken
				} else if parsed.AccessToken != "" {
					accessToken = parsed.AccessToken
				}
			}

			_ = livesync.SyncLiveAntigravityUI(targetPayload.Metadata.Email, targetPayload.Metadata.Name, accessToken, opts.QuotaGroups)
		}

		// Update app_storage.json last login username
		if targetPayload.Metadata.Email != "" {
			_ = livesync.UpdateAppStorageEmail(targetPayload.Metadata.Email)
		}

		// H. Update lastUsedAt
		targetPayload.Metadata.LastUsedAt = time.Now().UnixMilli()
		_ = profile.SaveProfile(targetName, *targetPayload)

		return nil
	}

	if err := execute(); err != nil {
		// ROLLBACK in reverse order
		for i := len(rollbackActions) - 1; i >= 0; i-- {
			rollbackActions[i]()
		}
		return nil, fmt.Errorf("[ATOMIC SWITCH FAILED & ROLLED BACK]: %w", err)
	}

	if opts.RestartLsp {
		_ = process.RestartLanguageServer()
		if targetPayload.Metadata.Email != "" {
			_ = livesync.WaitForLiveAntigravityReady(targetPayload.Metadata.Email, 4*time.Second)
		}
	}

	// Restore active conversation if one was captured prior to switch
	if savedConvoPath != "" {
		time.Sleep(300 * time.Millisecond)
		_ = livesync.RestoreConversation(savedConvoPath)
	}

	return &SwitchResult{
		Success:             true,
		ProfileName:         targetName,
		Email:               targetPayload.Metadata.Email,
		PreviousProfileName: previousActiveName,
	}, nil
}
