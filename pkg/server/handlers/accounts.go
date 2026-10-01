package handlers

import (
	"net/http"

	"agy-tools/pkg/config"
	"agy-tools/pkg/core/livesync"
	"agy-tools/pkg/store"
	"github.com/gin-gonic/gin"
)

func GetAccounts(c *gin.Context) {
	_ = store.DefaultStore.Load()
	accounts := store.DefaultStore.GetAccounts()

	// Trigger quota refresh for accounts missing quota groups in background
	for _, acc := range accounts {
		if acc.Quota == nil || len(acc.Quota.Groups) == 0 {
			accID := acc.ID
			go func() {
				_ = store.DefaultStore.RefreshQuota(accID)
			}()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts":         accounts,
		"currentAccountId": store.DefaultStore.GetCurrentAccountID(),
	})
}

func GetAntigravityStatus(c *gin.Context) {
	status, err := livesync.CheckAntigravityIdle()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"isRunning": status.IsRunning,
			"isIdle":    status.IsIdle,
			"reason":    status.Reason,
			"error":     err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"isRunning": status.IsRunning,
		"isIdle":    status.IsIdle,
		"reason":    status.Reason,
	})
}

func SwitchAntigravityAccount(c *gin.Context) {
	var req struct {
		ID    string `json:"id"`
		Force bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Missing account id"})
		return
	}

	cfg := config.LoadConfig()
	if !req.Force && cfg.Proxy.IdleDetectionEnabled {
		idleStatus, _ := livesync.CheckAntigravityIdle()
		if idleStatus.IsRunning && !idleStatus.IsIdle {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "Antigravity is currently busy (" + idleStatus.Reason + ").",
				"busy":    true,
				"reason":  idleStatus.Reason,
			})
			return
		}
	}

	acc, err := store.DefaultStore.SwitchAntigravityAccount(req.ID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "account": acc})
}

func RotateAntigravityAccount(c *gin.Context) {
	var req struct {
		Force  bool   `json:"force"`
		Prompt string `json:"prompt"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Prompt == "" {
		req.Prompt = "继续"
	}

	cfg := config.LoadConfig()
	if !req.Force && cfg.Proxy.IdleDetectionEnabled {
		idleStatus, _ := livesync.CheckAntigravityIdle()
		if idleStatus.IsRunning && !idleStatus.IsIdle {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "Antigravity is currently busy (" + idleStatus.Reason + ").",
				"busy":    true,
				"reason":  idleStatus.Reason,
			})
			return
		}
	}

	acc, err := store.DefaultStore.RotateAndResume(req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "account": acc, "prompt": req.Prompt})
}

func RotateAndResume(c *gin.Context) {
	RotateAntigravityAccount(c)
}

func RefreshAccount(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Missing id parameter"})
		return
	}

	if err := store.DefaultStore.RefreshAccount(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	_ = store.DefaultStore.RefreshQuota(id)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RefreshAllAccounts(c *gin.Context) {
	accounts := store.DefaultStore.GetAccounts()
	for _, acc := range accounts {
		_ = store.DefaultStore.RefreshAccount(acc.ID)
		_ = store.DefaultStore.RefreshQuota(acc.ID)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAccount(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Missing id parameter"})
		return
	}

	success := store.DefaultStore.RemoveAccount(id)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Account not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
