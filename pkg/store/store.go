package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"agy-tools/pkg/auth"
	"agy-tools/pkg/config"
	"agy-tools/pkg/core/credential"
	"agy-tools/pkg/core/livesync"
	"agy-tools/pkg/core/profile"
	"agy-tools/pkg/core/switcher"
	"agy-tools/pkg/quota"
	"agy-tools/pkg/types"
	"github.com/google/uuid"
)

type TokenStore struct {
	accounts         []types.Account
	currentAccountID *string
	mu               sync.RWMutex
}

var DefaultStore = &TokenStore{}

func init() {
	_ = DefaultStore.Load()
}

func (s *TokenStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = config.EnsureDirs()

	if _, err := os.Stat(config.AccountsFile); os.IsNotExist(err) {
		s.accounts = []types.Account{}
		s.currentAccountID = nil
		return nil
	}

	data, err := os.ReadFile(config.AccountsFile)
	if err != nil {
		s.accounts = []types.Account{}
		s.currentAccountID = nil
		return err
	}

	var index types.AccountIndex
	if err := json.Unmarshal(data, &index); err != nil {
		s.accounts = []types.Account{}
		s.currentAccountID = nil
		return err
	}

	s.accounts = index.Accounts
	s.currentAccountID = index.CurrentAccountID

	// Auto-detect current account if missing
	if s.currentAccountID == nil && len(s.accounts) > 0 {
		activeProf := profile.GetActiveProfileName()
		if activeProf != "" {
			if p, _ := profile.GetProfile(activeProf); p != nil {
				for _, a := range s.accounts {
					if strings.EqualFold(a.Email, p.Metadata.Email) || a.ID == p.Metadata.AccountID {
						s.currentAccountID = &a.ID
						break
					}
				}
			}
		}
		if s.currentAccountID == nil {
			s.currentAccountID = &s.accounts[0].ID
		}
	}

	return nil
}

func (s *TokenStore) Save() error {
	_ = config.EnsureDirs()
	index := types.AccountIndex{
		Accounts:         s.accounts,
		CurrentAccountID: s.currentAccountID,
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(config.AccountsFile, data, 0644)
}

func (s *TokenStore) GetAccounts() []types.Account {
	s.mu.RLock()
	if len(s.accounts) == 0 {
		s.mu.RUnlock()
		_ = s.Load()
		s.mu.RLock()
	}
	defer s.mu.RUnlock()
	copied := make([]types.Account, len(s.accounts))
	copy(copied, s.accounts)
	return copied
}

func (s *TokenStore) GetCurrentAccountID() *string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentAccountID
}

func (s *TokenStore) GetAccountByID(id string) *types.Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.accounts {
		if a.ID == id || strings.EqualFold(a.Email, id) {
			acc := a
			return &acc
		}
	}
	return nil
}

func (s *TokenStore) AddAccount(acc types.Account) *types.Account {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, a := range s.accounts {
		if strings.EqualFold(a.Email, acc.Email) {
			s.accounts[i].Tokens = acc.Tokens
			s.accounts[i].Name = acc.Name
			s.accounts[i].Tier = acc.Tier
			s.accounts[i].ProjectID = acc.ProjectID
			s.accounts[i].Disabled = false
			s.accounts[i].DisabledReason = ""
			_ = s.Save()
			res := s.accounts[i]
			return &res
		}
	}

	if acc.ID == "" {
		acc.ID = uuid.New().String()
	}
	if acc.CreatedAt == 0 {
		acc.CreatedAt = time.Now().UnixMilli()
	}

	s.accounts = append(s.accounts, acc)
	if s.currentAccountID == nil {
		s.currentAccountID = &acc.ID
	}
	_ = s.Save()

	return &acc
}

func (s *TokenStore) RemoveAccount(id string) bool {
	s.mu.Lock()

	foundIdx := -1
	for i, a := range s.accounts {
		if a.ID == id || strings.EqualFold(a.Email, id) {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		s.mu.Unlock()
		return false
	}

	wasActive := s.currentAccountID != nil && *s.currentAccountID == s.accounts[foundIdx].ID
	s.accounts = append(s.accounts[:foundIdx], s.accounts[foundIdx+1:]...)

	var nextAccountID string
	if wasActive {
		if len(s.accounts) > 0 {
			nextAccountID = s.accounts[0].ID
			s.currentAccountID = &nextAccountID
		} else {
			s.currentAccountID = nil
		}
	}
	_ = s.Save()
	s.mu.Unlock()

	// If the active account was removed, atomically switch Antigravity to the next available account
	if wasActive && nextAccountID != "" {
		_, _ = s.SwitchAntigravityAccount(nextAccountID, true)
	}

	return true
}

func (s *TokenStore) RefreshAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var account *types.Account
	for i, a := range s.accounts {
		if a.ID == id || strings.EqualFold(a.Email, id) {
			account = &s.accounts[i]
			break
		}
	}

	if account == nil {
		return fmt.Errorf("account not found: %s", id)
	}

	tokens, err := auth.RefreshTokens(account.Tokens.RefreshToken)
	if err != nil {
		account.Disabled = true
		account.DisabledReason = err.Error()
		_ = s.Save()
		return err
	}

	account.Tokens = *tokens
	account.Disabled = false
	account.DisabledReason = ""
	_ = s.Save()

	// Sync refreshed tokens to profile
	cleanEmail := strings.Split(account.Email, "@")[0]
	existing, _ := profile.ResolveProfile(cleanEmail)
	if existing == nil {
		existing, _ = profile.ResolveProfile(account.Email)
	}
	if existing != nil {
		expiryIso := time.UnixMilli(tokens.ExpiresAt).Format(time.RFC3339)
		jetskiPayload := map[string]interface{}{
			"token": map[string]interface{}{
				"access_token":  tokens.AccessToken,
				"token_type":    "Bearer",
				"refresh_token": tokens.RefreshToken,
				"expiry":        expiryIso,
			},
			"auth_method": "consumer",
		}
		if tokens.IDToken != "" {
			jetskiPayload["id_token"] = tokens.IDToken
		}
		tokenBytes, _ := json.MarshalIndent(jetskiPayload, "", "  ")

		_ = profile.SaveProfile(existing.Metadata.Name, types.ProfilePayload{
			InstallationID:  existing.InstallationID,
			TokenPayload:    string(tokenBytes),
			SettingsPayload: existing.SettingsPayload,
			Metadata: types.ProfileMetadata{
				Name:       existing.Metadata.Name,
				Email:      account.Email,
				AccountID:  account.ID,
				Tier:       account.Tier,
				LastUsedAt: time.Now().UnixMilli(),
				CreatedAt:  existing.Metadata.CreatedAt,
			},
		})

		if profile.GetActiveProfileName() == existing.Metadata.Name {
			_ = credential.Write(types.CredentialTarget, string(tokenBytes))
			_ = os.WriteFile(config.PrimaryTokenFile, tokenBytes, 0644)
			_ = os.WriteFile(config.SecondaryTokenFile, tokenBytes, 0644)
			_ = os.WriteFile(config.AntigravityDirTokenFile, tokenBytes, 0644)
			_ = os.WriteFile(config.AntigravityDirAltTokenFile, tokenBytes, 0644)
		}
	}

	return nil
}

func (s *TokenStore) RefreshQuota(id string) error {
	var accessToken, projectID, email, accountID string
	var expiresAt int64

	s.mu.RLock()
	for _, a := range s.accounts {
		if a.ID == id || strings.EqualFold(a.Email, id) {
			accessToken = a.Tokens.AccessToken
			projectID = a.ProjectID
			email = a.Email
			accountID = a.ID
			expiresAt = a.Tokens.ExpiresAt
			break
		}
	}
	s.mu.RUnlock()

	if accessToken == "" {
		return fmt.Errorf("account not found: %s", id)
	}

	// Auto-refresh token if within 30s of expiry
	if expiresAt-time.Now().UnixMilli() < 30000 {
		_ = s.RefreshAccount(accountID)
		s.mu.RLock()
		for _, a := range s.accounts {
			if a.ID == accountID {
				accessToken = a.Tokens.AccessToken
				break
			}
		}
		s.mu.RUnlock()
	}

	qData, err := quota.FetchQuota(accessToken, projectID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.accounts {
		if a.ID == accountID {
			s.accounts[i].Quota = qData
			_ = s.Save()
			break
		}
	}

	_ = email
	return nil
}

func (s *TokenStore) RefreshAllQuotas() {
	accounts := s.GetAccounts()
	for _, a := range accounts {
		_ = s.RefreshQuota(a.ID)
	}
}

func (s *TokenStore) SwitchAntigravityAccount(id string, skipProcessCheck bool) (*types.Account, error) {
	_ = s.Load()

	var account *types.Account
	s.mu.RLock()
	for _, a := range s.accounts {
		if a.ID == id || strings.EqualFold(a.Email, id) {
			acc := a
			account = &acc
			break
		}
	}
	s.mu.RUnlock()

	if account == nil {
		return nil, fmt.Errorf("account not found: %s", id)
	}

	// Refresh token if within 5 mins of expiry
	if account.Tokens.ExpiresAt-time.Now().UnixMilli() < 5*60*1000 {
		_ = s.RefreshAccount(account.ID)
		acc := s.GetAccountByID(account.ID)
		if acc != nil {
			account = acc
		}
	}

	// Format jetski token payload
	expiryIso := time.UnixMilli(account.Tokens.ExpiresAt).Format(time.RFC3339)
	jetskiPayload := map[string]interface{}{
		"token": map[string]interface{}{
			"access_token":  account.Tokens.AccessToken,
			"token_type":    "Bearer",
			"refresh_token": account.Tokens.RefreshToken,
			"expiry":        expiryIso,
		},
		"auth_method": "consumer",
	}
	if account.Tokens.IDToken != "" {
		jetskiPayload["id_token"] = account.Tokens.IDToken
	}
	tokenBytes, _ := json.MarshalIndent(jetskiPayload, "", "  ")

	profileName := strings.Split(account.Email, "@")[0]
	existing, resolvedName := profile.ResolveProfile(profileName)
	if existing == nil {
		existing, resolvedName = profile.ResolveProfile(account.Email)
	}

	targetProfileName := profileName
	if resolvedName != "" {
		targetProfileName = resolvedName
	}

	if existing == nil {
		_ = profile.SaveProfile(targetProfileName, types.ProfilePayload{
			InstallationID:  uuid.New().String(),
			TokenPayload:    string(tokenBytes),
			SettingsPayload: `{"mcpServers":{},"security":{"auth":{"selectedType":"oauth-personal"}}}`,
			Metadata: types.ProfileMetadata{
				Name:       targetProfileName,
				Email:      account.Email,
				AccountID:  account.ID,
				Tier:       account.Tier,
				LastUsedAt: time.Now().UnixMilli(),
				CreatedAt:  time.Now().UnixMilli(),
			},
		})
	} else {
		_ = profile.SaveProfile(targetProfileName, types.ProfilePayload{
			InstallationID:  existing.InstallationID,
			TokenPayload:    string(tokenBytes),
			SettingsPayload: existing.SettingsPayload,
			Metadata: types.ProfileMetadata{
				Name:       targetProfileName,
				Email:      account.Email,
				AccountID:  account.ID,
				Tier:       account.Tier,
				LastUsedAt: time.Now().UnixMilli(),
				CreatedAt:  existing.Metadata.CreatedAt,
			},
		})
	}

	var quotaGroups []types.QuotaGroup
	if account.Quota != nil {
		quotaGroups = account.Quota.Groups
	}

	// Perform atomic switch: restarts language_server to ensure quota attribution belongs to new account
	_, err := switcher.SwitchProfile(targetProfileName, switcher.SwitchOptions{
		SkipProcessCheck: skipProcessCheck,
		RestartLsp:       true,
		QuotaGroups:      quotaGroups,
	})
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.currentAccountID = &account.ID
	_ = s.Save()
	s.mu.Unlock()

	// Refresh quota in background
	go func() {
		_ = s.RefreshQuota(account.ID)
	}()

	return account, nil
}

func (s *TokenStore) RotateAntigravityAccount(skipProcessCheck bool) (*types.Account, error) {
	_ = s.Load()

	accounts := s.GetAccounts()
	var eligible []types.Account
	for _, a := range accounts {
		if !a.Disabled {
			eligible = append(eligible, a)
		}
	}

	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible accounts to rotate to")
	}

	currentID := s.GetCurrentAccountID()
	currIdx := -1
	if currentID != nil {
		for i, a := range eligible {
			if a.ID == *currentID {
				currIdx = i
				break
			}
		}
	}

	nextIdx := (currIdx + 1) % len(eligible)
	nextAccount := eligible[nextIdx]

	return s.SwitchAntigravityAccount(nextAccount.ID, skipProcessCheck)
}

// RotateAndResume 专为“额度用尽”场景设计：
// 1. 记忆当前活跃会话路径（如 /c/<id>?section=...）；
// 2. 顺位轮换至下一个健康有效的 Google 账号，原子写入凭据并重载语言服务；
// 3. 自动切回原会话路径，确保工作现场不丢失；
// 4. 定位聊天输入框，注入文本（默认“继续”）并立即直接发送（DOM 发送按钮点击 + CDP 原生 Enter 物理按键双重保证）。
func (s *TokenStore) RotateAndResume(prompt string) (*types.Account, error) {
	if prompt == "" {
		prompt = "继续"
	}

	// 1. 记忆当前活跃的会话路径
	convoPath := livesync.GetCurrentConversationPath()

	// 2. 顺位轮换账号并重载服务
	acc, err := s.RotateAntigravityAccount(true)
	if err != nil {
		return nil, err
	}

	// 3. 切回原会话
	if convoPath != "" {
		_ = livesync.RestoreConversation(convoPath)
	}

	// 4. 异步等待原会话 DOM 就绪后直接发送“继续”
	go func() {
		// 等待界面完成会话恢复与输入框挂载
		time.Sleep(1500 * time.Millisecond)
		ok := livesync.SendChatMessage(prompt)
		// 如果首次因 DOM 动画未完成未能成功发送，在 1 秒后重试一次保障发送
		if !ok {
			time.Sleep(1000 * time.Millisecond)
			_ = livesync.SendChatMessage(prompt)
		}
	}()

	return acc, nil
}

func (s *TokenStore) GetAccount(idOrEmail string) *types.Account {
	return s.GetAccountByID(idOrEmail)
}

func (s *TokenStore) GetActiveAccount() *types.Account {
	id := s.GetCurrentAccountID()
	if id == nil {
		return nil
	}
	return s.GetAccountByID(*id)
}

func (s *TokenStore) RefreshAllAccounts() {
	accounts := s.GetAccounts()
	for _, a := range accounts {
		_ = s.RefreshAccount(a.ID)
		_ = s.RefreshQuota(a.ID)
	}
}

type QuotaRefresher struct {
	cancel context.CancelFunc
	mu     sync.Mutex
}

var DefaultQuotaRefresher = &QuotaRefresher{}

func (q *QuotaRefresher) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.cancel != nil {
		q.cancel()
	}

	cfg := config.LoadConfig()
	if !cfg.Proxy.AutoRefreshQuota {
		return
	}

	intervalMinutes := cfg.Proxy.QuotaRefreshIntervalMinutes
	if intervalMinutes <= 0 {
		intervalMinutes = 5
	}
	interval := time.Duration(intervalMinutes) * time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	q.cancel = cancel

	go func() {
		// Run initial refresh shortly after startup
		time.Sleep(2 * time.Second)
		DefaultStore.RefreshAllQuotas()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentCfg := config.LoadConfig()
				if currentCfg.Proxy.AutoRefreshQuota {
					DefaultStore.RefreshAllQuotas()
					DefaultStore.CheckAndAutoRotateIfQuotaExhausted()
				}
			}
		}
	}()
}

// CheckAndAutoRotateIfQuotaExhausted checks if the currently active account has depleted its quota,
// and if AutoRotateOnQuotaExhausted is enabled, automatically switches to the next account and continues.
func (s *TokenStore) CheckAndAutoRotateIfQuotaExhausted() {
	cfg := config.LoadConfig()
	if !cfg.Proxy.AutoRotateOnQuotaExhausted {
		return
	}

	activeAcc := s.GetActiveAccount()
	if activeAcc == nil || activeAcc.Quota == nil {
		return
	}

	hasBuckets := false
	allDepleted := true
	for _, group := range activeAcc.Quota.Groups {
		for _, b := range group.Buckets {
			hasBuckets = true
			if b.RemainingFraction > 0.01 || b.Percentage > 0.01 {
				allDepleted = false
				break
			}
		}
		if !allDepleted {
			break
		}
	}

	if hasBuckets && allDepleted {
		accounts := s.GetAccounts()
		eligibleCount := 0
		for _, a := range accounts {
			if !a.Disabled && a.ID != activeAcc.ID {
				eligibleCount++
			}
		}
		if eligibleCount > 0 {
			_, _ = s.RotateAndResume("继续")
		}
	}
}

func (s *TokenStore) StartBackgroundQuotaFetch() {
	DefaultQuotaRefresher.Start()
}

