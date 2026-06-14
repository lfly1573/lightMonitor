package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lightmonitor/internal/application/core"
	"lightmonitor/internal/application/install"
	"lightmonitor/internal/infrastructure/http/handler"
	"lightmonitor/internal/infrastructure/http/middleware"
	webassets "lightmonitor/web"
)

type Dependencies struct {
	InstallService *install.Service
	CoreService    *core.Service
}

type Server struct {
	router *gin.Engine
}

func NewServer(deps Dependencies) *Server {
	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestID())

	sessions := handler.NewSessionManager()
	installHandler := handler.NewInstallHandler(deps.InstallService)
	apiHandler := handler.NewAPIHandler(deps.CoreService, sessions)

	router.GET("/api/health", apiHandler.Health)
	router.GET("/api/install/status", installHandler.Status)
	router.POST("/api/install", installHandler.Install)
	router.POST("/api/auth/login", apiHandler.Login)
	router.POST("/api/v1/receive", apiHandler.Receive)

	api := router.Group("/api")
	api.Use(sessions.Require())
	api.GET("/auth/me", apiHandler.Me)
	api.POST("/auth/logout", apiHandler.Logout)
	api.GET("/dashboard", apiHandler.Dashboard)
	api.GET("/settings", apiHandler.Settings)
	api.GET("/users", apiHandler.Users)
	api.GET("/groups", apiHandler.Groups)
	api.GET("/items", apiHandler.Items)
	api.GET("/active-requests", apiHandler.ActiveRequests)
	api.GET("/fields", apiHandler.Fields)
	api.GET("/channels", apiHandler.Channels)
	api.GET("/rules", apiHandler.Rules)
	api.GET("/samples", apiHandler.Samples)
	api.GET("/stats", apiHandler.Stats)
	api.GET("/events", apiHandler.Events)

	admin := api.Group("")
	admin.Use(handler.RequireAdmin())
	admin.PUT("/settings", apiHandler.UpdateSettings)
	admin.POST("/users", apiHandler.CreateUser)
	admin.PUT("/users/:id", apiHandler.UpdateUser)
	admin.DELETE("/users/:id", apiHandler.DeleteUser)
	admin.POST("/groups", apiHandler.CreateGroup)
	admin.PUT("/groups/:id", apiHandler.UpdateGroup)
	admin.DELETE("/groups/:id", apiHandler.DeleteGroup)
	admin.POST("/items", apiHandler.CreateItem)
	admin.PUT("/items/:id", apiHandler.UpdateItem)
	admin.DELETE("/items/:id", apiHandler.DeleteItem)
	admin.POST("/active-requests", apiHandler.CreateActiveRequest)
	admin.PUT("/active-requests/:id", apiHandler.UpdateActiveRequest)
	admin.DELETE("/active-requests/:id", apiHandler.DeleteActiveRequest)
	admin.POST("/fields", apiHandler.UpsertField)
	admin.DELETE("/fields/:id", apiHandler.DeleteField)
	admin.POST("/channels", apiHandler.UpsertChannel)
	admin.DELETE("/channels/:id", apiHandler.DeleteChannel)
	admin.POST("/rules", apiHandler.UpsertRule)
	admin.DELETE("/rules/:id", apiHandler.DeleteRule)

	webassets.RegisterRoutes(router)

	return &Server{router: router}
}

func (s *Server) Run(addr string) error {
	return http.ListenAndServe(addr, s.router)
}
