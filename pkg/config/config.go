package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var (
	homeDir, _ = os.UserHomeDir()
	appData    = os.Getenv("APPDATA")

	// Global paths
	AgyAuthDir     = filepath.Join(homeDir, ".agy_auth")
	ProfilesDir    = filepath.Join(AgyAuthDir, "profiles")
	SharedDir      = filepath.Join(AgyAuthDir, "shared")
	ActiveMarker   = filepath.Join(AgyAuthDir, "active")

	// Tool config paths
	AgyToolsDir  = filepath.Join(homeDir, ".agy-tools")
	ConfigFile   = filepath.Join(AgyToolsDir, "config.json")
	AccountsFile = filepath.Join(AgyToolsDir, "accounts.json")

	// Antigravity core files
	GeminiDir                   = filepath.Join(homeDir, ".gemini")
	PrimaryInstallationIDFile   = filepath.Join(GeminiDir, "antigravity-browser-profile-installation-id")
	PrimaryTokenFile            = filepath.Join(GeminiDir, "jetski-standalone-oauth-token")
	SecondaryTokenFile          = filepath.Join(GeminiDir, "antigravity-oauth-token")
	AntigravityDirTokenFile     = filepath.Join(GeminiDir, "antigravity", "oauth_token")
	AntigravityDirAltTokenFile  = filepath.Join(GeminiDir, "antigravity", "antigravity-oauth-token")
	PrimarySettingsFile         = filepath.Join(GeminiDir, "antigravity", "settings.json")
	AntigravityDirSettingsFile  = filepath.Join(GeminiDir, "settings.json")
	AntigravityIDEDir           = filepath.Join(GeminiDir, "antigravity-ide")
	IDEInstallationIDFile       = filepath.Join(AntigravityIDEDir, "installation_id")
	AntigravityDirInstallIDFile = filepath.Join(GeminiDir, "antigravity", "installation_id")
	AntigravityStateFile        = filepath.Join(GeminiDir, "antigravity", "antigravity_state.pbtxt")

	// Electron App Data & DevTools
	AntigravityAppDataDir = filepath.Join(appData, "Antigravity")
	AppStorageFile        = filepath.Join(AntigravityAppDataDir, "User", "globalStorage", "app_storage.json")
	DevToolsPortFile      = filepath.Join(AntigravityAppDataDir, "DevToolsActivePort")

	// Shared links targets
	SharedConversationsDir = filepath.Join(SharedDir, "conversations")
	SharedSkillsDir        = filepath.Join(SharedDir, "skills")

	// Symlink origins
	ConversationsPrimary = filepath.Join(GeminiDir, "antigravity", "conversations")
	ConversationsIDE     = filepath.Join(AntigravityIDEDir, "conversations")
	SkillsConfig         = filepath.Join(GeminiDir, "config", "skills")
	SkillsAntigravity    = filepath.Join(GeminiDir, "antigravity", "skills")
)

type ProxyConfig struct {
	Port                        int      `json:"port"`
	Host                        string   `json:"host"`
	AutoRefreshQuota            bool     `json:"autoRefreshQuota"`
	QuotaRefreshIntervalMinutes int      `json:"quotaRefreshIntervalMinutes"`
	IdleDetectionEnabled        bool     `json:"idleDetectionEnabled"`
	AutoRotateOnQuotaExhausted  bool     `json:"autoRotateOnQuotaExhausted"`
	ModelFamilyPriority         []string `json:"modelFamilyPriority"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type Config struct {
	Proxy LogConfigProxy `json:"proxy"`
	Log   LogConfig      `json:"log"`
}

type LogConfigProxy struct {
	ProxyConfig
}

var (
	currentConfig Config
	configMu      sync.RWMutex
)

func DefaultConfig() Config {
	return Config{
		Proxy: LogConfigProxy{
			ProxyConfig: ProxyConfig{
				Port:                        38080,
				Host:                        "127.0.0.1",
				AutoRefreshQuota:            true,
				QuotaRefreshIntervalMinutes: 5,
				IdleDetectionEnabled:        true,
				AutoRotateOnQuotaExhausted:  true,
				ModelFamilyPriority:         []string{"claude", "gemini"},
			},
		},
		Log: LogConfig{
			Level: "info",
		},
	}
}

func EnsureDirs() error {
	dirs := []string{
		AgyAuthDir,
		ProfilesDir,
		SharedDir,
		SharedConversationsDir,
		SharedSkillsDir,
		AgyToolsDir,
		GeminiDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

func LoadConfig() Config {
	configMu.Lock()
	defer configMu.Unlock()

	_ = EnsureDirs()

	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		currentConfig = DefaultConfig()
		saveConfigFile(currentConfig)
		return currentConfig
	}

	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		currentConfig = DefaultConfig()
		return currentConfig
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		currentConfig = DefaultConfig()
		return currentConfig
	}

	// Set defaults if empty
	if cfg.Proxy.Port == 0 {
		cfg.Proxy.Port = 38080
	}
	if cfg.Proxy.Host == "" {
		cfg.Proxy.Host = "127.0.0.1"
	}
	if cfg.Proxy.QuotaRefreshIntervalMinutes == 0 {
		cfg.Proxy.QuotaRefreshIntervalMinutes = 5
	}

	currentConfig = cfg
	return currentConfig
}

func UpdateConfig(patch Config) Config {
	configMu.Lock()
	defer configMu.Unlock()

	// Update fields
	if patch.Proxy.Port != 0 {
		currentConfig.Proxy.Port = patch.Proxy.Port
	}
	if patch.Proxy.Host != "" {
		currentConfig.Proxy.Host = patch.Proxy.Host
	}
	if patch.Proxy.QuotaRefreshIntervalMinutes > 0 {
		currentConfig.Proxy.QuotaRefreshIntervalMinutes = patch.Proxy.QuotaRefreshIntervalMinutes
	} else if currentConfig.Proxy.QuotaRefreshIntervalMinutes == 0 {
		currentConfig.Proxy.QuotaRefreshIntervalMinutes = 5
	}

	currentConfig.Proxy.AutoRefreshQuota = patch.Proxy.AutoRefreshQuota
	currentConfig.Proxy.IdleDetectionEnabled = patch.Proxy.IdleDetectionEnabled
	currentConfig.Proxy.AutoRotateOnQuotaExhausted = patch.Proxy.AutoRotateOnQuotaExhausted

	saveConfigFile(currentConfig)
	return currentConfig
}

func saveConfigFile(cfg Config) {
	_ = EnsureDirs()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(ConfigFile, data, 0644)
}
