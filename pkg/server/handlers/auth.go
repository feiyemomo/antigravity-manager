package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"agy-tools/pkg/auth"
	"agy-tools/pkg/quota"
	"agy-tools/pkg/store"
	"agy-tools/pkg/types"
	"github.com/gin-gonic/gin"
)

type LoginState struct {
	Status  string         `json:"status"` // "idle", "waiting", "success", "error"
	Account *types.Account `json:"account,omitempty"`
	Error   string         `json:"error,omitempty"`
}

var (
	loginState LoginState = LoginState{Status: "idle"}
	loginMu    sync.RWMutex
)

func StartLogin(c *gin.Context) {
	loginMu.Lock()
	loginState = LoginState{Status: "waiting"}
	loginMu.Unlock()

	state := fmt.Sprintf("%d", time.Now().UnixNano())
	authURL := auth.GenerateAuthURL(state)

	c.JSON(http.StatusOK, gin.H{
		"authUrl": authURL,
		"status":  "waiting",
	})
}

func GetLoginStatus(c *gin.Context) {
	loginMu.RLock()
	defer loginMu.RUnlock()
	c.JSON(http.StatusOK, loginState)
}

func OAuthCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		errStr := c.Query("error")
		if errStr == "" {
			errStr = "missing authorization code"
		}
		loginMu.Lock()
		loginState = LoginState{Status: "error", Error: errStr}
		loginMu.Unlock()
		c.String(http.StatusBadRequest, "OAuth Error: "+errStr)
		return
	}

	tokens, err := auth.ExchangeCode(code)
	if err != nil {
		loginMu.Lock()
		loginState = LoginState{Status: "error", Error: err.Error()}
		loginMu.Unlock()
		c.String(http.StatusInternalServerError, "Token exchange failed: "+err.Error())
		return
	}

	email, name, err := auth.FetchUserInfo(tokens.AccessToken)
	if err != nil {
		loginMu.Lock()
		loginState = LoginState{Status: "error", Error: err.Error()}
		loginMu.Unlock()
		c.String(http.StatusInternalServerError, "Failed to fetch user info: "+err.Error())
		return
	}

	projectID, tier := auth.FetchProjectIDAndTier(tokens.AccessToken)
	qData, _ := quota.FetchQuota(tokens.AccessToken, projectID)

	acc := types.Account{
		Email:     email,
		Name:      name,
		Tokens:    *tokens,
		ProjectID: projectID,
		Tier:      tier,
		Quota:     qData,
	}

	saved := store.DefaultStore.AddAccount(acc)

	loginMu.Lock()
	loginState = LoginState{
		Status:  "success",
		Account: saved,
	}
	loginMu.Unlock()

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `<html><body style="font-family:sans-serif;text-align:center;padding:50px;background:#0f172a;color:#fff;">
		<h2>🎉 授权成功！</h2>
		<p>Google 账户已成功连接至 Antigravity 控制台。您可以关闭此页面返回控制台。</p>
		<script>setTimeout(() => window.close(), 2500);</script>
	</body></html>`)
}
