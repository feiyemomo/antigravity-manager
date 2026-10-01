package proxy

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ModelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

var AvailableModels = []string{
	"gemini-2.5-pro",
	"gemini-2.5-flash",
	"gemini-3-pro",
	"gemini-3-flash",
	"gemini-3.1-pro-high",
	"gemini-3.8-flash-high",
	"gemini-3.8-flash-medium",
	"claude-3-7-sonnet",
	"claude-sonnet-4-6",
	"claude-opus-4-6",
	"gpt-oss-120b-medium",
}

func HandleListModels(c *gin.Context) {
	now := time.Now().Unix()
	var list []ModelItem
	for _, m := range AvailableModels {
		list = append(list, ModelItem{
			ID:      m,
			Object:  "model",
			Created: now,
			OwnedBy: "google-antigravity",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   list,
	})
}
