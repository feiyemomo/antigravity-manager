package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agy-tools/pkg/config"
	"agy-tools/pkg/types"
	"github.com/google/uuid"
)

// GetActiveProfileName reads the active profile name from active marker file
func GetActiveProfileName() string {
	data, err := os.ReadFile(config.ActiveMarker)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetActiveProfileName writes the active profile name to the active marker file
func SetActiveProfileName(name string) error {
	_ = config.EnsureDirs()
	return os.WriteFile(config.ActiveMarker, []byte(name), 0644)
}

// ListProfiles lists all configured profile metadata
func ListProfiles() ([]types.ProfileMetadata, error) {
	_ = config.EnsureDirs()
	entries, err := os.ReadDir(config.ProfilesDir)
	if err != nil {
		return nil, err
	}

	var list []types.ProfileMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		payload, err := GetProfile(name)
		if err == nil && payload != nil {
			list = append(list, payload.Metadata)
		}
	}

	return list, nil
}

// GetProfile reads the full profile payload from ~/.agy_auth/profiles/<name>/
func GetProfile(name string) (*types.ProfilePayload, error) {
	profileDir := filepath.Join(config.ProfilesDir, name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile '%s' does not exist", name)
	}

	installIDBytes, _ := os.ReadFile(filepath.Join(profileDir, "installation_id"))
	tokenBytes, _ := os.ReadFile(filepath.Join(profileDir, "antigravity-oauth-token"))
	settingsBytes, _ := os.ReadFile(filepath.Join(profileDir, "settings.json"))

	var meta types.ProfileMetadata
	metaBytes, err := os.ReadFile(filepath.Join(profileDir, "profile.json"))
	if err == nil {
		_ = json.Unmarshal(metaBytes, &meta)
	}
	if meta.Name == "" {
		meta.Name = name
	}

	return &types.ProfilePayload{
		InstallationID:  strings.TrimSpace(string(installIDBytes)),
		TokenPayload:    string(tokenBytes),
		SettingsPayload: string(settingsBytes),
		Metadata:        meta,
	}, nil
}

// SaveProfile writes the isolated files and metadata for the profile
func SaveProfile(name string, payload types.ProfilePayload) error {
	_ = config.EnsureDirs()
	profileDir := filepath.Join(config.ProfilesDir, name)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return err
	}

	if payload.InstallationID == "" {
		payload.InstallationID = uuid.New().String()
	}

	if payload.Metadata.Name == "" {
		payload.Metadata.Name = name
	}
	if payload.Metadata.CreatedAt == 0 {
		payload.Metadata.CreatedAt = time.Now().UnixMilli()
	}
	payload.Metadata.LastUsedAt = time.Now().UnixMilli()

	_ = os.WriteFile(filepath.Join(profileDir, "installation_id"), []byte(payload.InstallationID), 0644)
	_ = os.WriteFile(filepath.Join(profileDir, "antigravity-oauth-token"), []byte(payload.TokenPayload), 0644)
	_ = os.WriteFile(filepath.Join(profileDir, "settings.json"), []byte(payload.SettingsPayload), 0644)

	metaBytes, _ := json.MarshalIndent(payload.Metadata, "", "  ")
	_ = os.WriteFile(filepath.Join(profileDir, "profile.json"), metaBytes, 0644)

	return nil
}

// DeleteProfile deletes a profile directory
func DeleteProfile(name string) error {
	profileDir := filepath.Join(config.ProfilesDir, name)
	return os.RemoveAll(profileDir)
}

// ResolveProfile finds a profile matching name, email, or account ID
func ResolveProfile(identifier string) (*types.ProfilePayload, string) {
	if payload, err := GetProfile(identifier); err == nil {
		return payload, identifier
	}

	profiles, err := ListProfiles()
	if err != nil {
		return nil, ""
	}

	lowerId := strings.ToLower(identifier)
	for _, p := range profiles {
		if strings.EqualFold(p.Name, identifier) ||
			strings.EqualFold(p.Email, lowerId) ||
			p.AccountID == identifier {
			payload, err := GetProfile(p.Name)
			if err == nil {
				return payload, p.Name
			}
		}
	}

	return nil, ""
}
