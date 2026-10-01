package handlers

import (
	"net/http"

	"agy-tools/pkg/config"
	"agy-tools/pkg/store"
	"github.com/gin-gonic/gin"
)

func GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, config.LoadConfig())
}

func UpdateConfig(c *gin.Context) {
	var patch config.Config
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	updated := config.UpdateConfig(patch)
	store.DefaultQuotaRefresher.Start()

	c.JSON(http.StatusOK, gin.H{"success": true, "config": updated})
}
