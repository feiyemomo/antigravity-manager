package server

import (
	"fmt"
	"net/http"

	"agy-tools/pkg/server/handlers"
	"agy-tools/pkg/server/proxy"
	"agy-tools/pkg/server/web"
	"agy-tools/pkg/types"
	"github.com/gin-gonic/gin"
)

// CorsMiddleware handles cross-origin requests
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SetupEngine creates and configures the Gin engine
func SetupEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CorsMiddleware())

	// Embedded Web Dashboard
	r.GET("/", func(c *gin.Context) {
		data, err := web.Content.ReadFile("index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Dashboard template not found: "+err.Error())
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// OAuth callback endpoint (handles both port 8976 and port 38080)
	r.GET("/callback", handlers.OAuthCallback)

	// API Routes Group
	api := r.Group("/api")
	{
		api.GET("/accounts", handlers.GetAccounts)
		api.GET("/antigravity/status", handlers.GetAntigravityStatus)
		api.POST("/antigravity/switch", handlers.SwitchAntigravityAccount)
		api.POST("/antigravity/rotate", handlers.RotateAntigravityAccount)
		api.POST("/antigravity/rotate-and-resume", handlers.RotateAndResume)
		api.GET("/config", handlers.GetConfig)
		api.POST("/config", handlers.UpdateConfig)
		api.POST("/accounts/:id/refresh", handlers.RefreshAccount)
		api.POST("/accounts/refresh-all", handlers.RefreshAllAccounts)
		api.DELETE("/accounts/:id", handlers.DeleteAccount)
		api.GET("/login/start", handlers.StartLogin)
		api.GET("/login/status", handlers.GetLoginStatus)
	}

	// OpenAI / Compatibility Proxy Routes
	v1 := r.Group("/v1")
	{
		v1.GET("/models", proxy.HandleListModels)
	}

	return r
}

// StartServer runs the Gin web server
func StartServer(port int, host string) error {
	engine := SetupEngine()

	// Also listen on OAuth redirect port (8976) so Google OAuth redirects are captured seamlessly
	if port != types.OAuthRedirectPort {
		go func() {
			oauthAddr := fmt.Sprintf("127.0.0.1:%d", types.OAuthRedirectPort)
			srv := &http.Server{
				Addr:    oauthAddr,
				Handler: engine,
			}
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("[OAuth Server] Info: Port %d: %v\n", types.OAuthRedirectPort, err)
			}
		}()
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	return engine.Run(addr)
}
