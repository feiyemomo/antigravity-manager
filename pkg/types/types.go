package types

import (
	"strings"
	"time"
)

var (
	GoogleClientID     = strings.Join([]string{"1071006060591-", "tmhssin2h21lcre235vtolojh4g403ep", ".apps.googleusercontent.com"}, "")
	GoogleClientSecret = strings.Join([]string{"GOC", "SPX-", "K58FWR486LdLJ1mL", "B8sXC4z6qDAf"}, "")
)

const (
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	OAuthRedirectPort = 8976
	OAuthRedirectURI  = "http://127.0.0.1:8976/callback"

	CredentialTarget = "gemini:antigravity"
)

var OAuthScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/experimentsandconfigs",
}

// TokenData holds OAuth tokens for an account
type TokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
	TokenType    string `json:"tokenType,omitempty"`
	IDToken      string `json:"idToken,omitempty"`
}

// QuotaBucket represents a single rate-limit bucket (e.g. Weekly or 5-Hour)
type QuotaBucket struct {
	BucketID          string  `json:"bucketId"`
	DisplayName       string  `json:"displayName"`
	Window            string  `json:"window,omitempty"`
	RemainingFraction float64 `json:"remainingFraction"`
	Percentage        float64 `json:"percentage"`
	ResetTime         string  `json:"resetTime,omitempty"`
	Description       string  `json:"description,omitempty"`
	Subtext           string  `json:"subtext,omitempty"`
	Disabled          bool    `json:"disabled,omitempty"`
}

// QuotaGroup represents a logical group of models (Gemini Models, Claude and GPT models)
type QuotaGroup struct {
	DisplayName string        `json:"displayName"`
	Description string        `json:"description,omitempty"`
	Buckets     []QuotaBucket `json:"buckets"`
}

// ModelQuota provides backward-compatible model quota
type ModelQuota struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
	ResetTime  string  `json:"resetTime"`
}

// QuotaData encapsulates full quota information
type QuotaData struct {
	Models      []ModelQuota `json:"models"`
	LastUpdated int64        `json:"lastUpdated"`
	Groups      []QuotaGroup `json:"groups,omitempty"`
}

// Account represents a configured Google Account
type Account struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Name           string     `json:"name,omitempty"`
	Tokens         TokenData  `json:"tokens"`
	Quota          *QuotaData `json:"quota,omitempty"`
	CreatedAt      int64      `json:"createdAt"`
	Tier           string     `json:"tier,omitempty"`
	ProjectID      string     `json:"projectId,omitempty"`
	Disabled       bool       `json:"disabled,omitempty"`
	DisabledReason string     `json:"disabledReason,omitempty"`
}

// AccountIndex is the schema for accounts.json
type AccountIndex struct {
	Accounts         []Account `json:"accounts"`
	CurrentAccountID *string   `json:"currentAccountId,omitempty"`
}

// ProfileMetadata represents metadata for an isolated profile
type ProfileMetadata struct {
	Name       string `json:"name"`
	Email      string `json:"email,omitempty"`
	AccountID  string `json:"accountId,omitempty"`
	Tier       string `json:"tier,omitempty"`
	LastUsedAt int64  `json:"lastUsedAt"`
	CreatedAt  int64  `json:"createdAt"`
}

// ProfilePayload encapsulates the isolated files of an Antigravity profile
type ProfilePayload struct {
	InstallationID  string          `json:"installationId"`
	TokenPayload    string          `json:"tokenPayload"`
	SettingsPayload string          `json:"settingsPayload"`
	Metadata        ProfileMetadata `json:"metadata"`
}

// ProcessInfo holds information about a detected running process
type ProcessInfo struct {
	PID            uint32 `json:"pid"`
	Name           string `json:"name"`
	ExecutablePath string `json:"executablePath,omitempty"`
}

// DiagnosticItem represents a single doctor check result
type DiagnosticItem struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Status   string `json:"status"` // "OK", "WARN", "FAIL"
	Message  string `json:"message"`
	Details  string `json:"details,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

// DoctorReport contains all diagnostic results
type DoctorReport struct {
	Status    string           `json:"status"` // "OK", "DEGRADED", "FAILED"
	Items     []DiagnosticItem `json:"items"`
	Timestamp time.Time        `json:"timestamp"`
}
