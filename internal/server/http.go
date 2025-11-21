package server

import (
	"context"
	"fmt"
	"net/http"

	"linkmaster-node/internal/config"
	"linkmaster-node/internal/handler"
	"linkmaster-node/internal/recovery"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type HTTPServer struct {
	server *http.Server
	logger *zap.Logger
}

func NewHTTPServer(cfg *config.Config) *HTTPServer {
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(recoveryMiddleware)

	// 初始化持续测试处理器
	handler.InitContinuousHandler(cfg)
	
	// 启动任务清理goroutine
	handler.StartTaskCleanup()

	// 注册路由
	api := router.Group("/api")
	{
		api.POST("/test", handler.HandleTest)
		api.POST("/continuous/start", handler.HandleContinuousStart)
		api.POST("/continuous/stop", handler.HandleContinuousStop)
		api.GET("/continuous/status", handler.HandleContinuousStatus)
		api.GET("/health", handler.HandleHealth)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	logger, _ := zap.NewProduction()

	return &HTTPServer{
		server: server,
		logger: logger,
	}
}

func (s *HTTPServer) Start() error {
	s.logger.Info("HTTP服务器启动", zap.String("addr", s.server.Addr))
	return s.server.ListenAndServe()
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func recoveryMiddleware(c *gin.Context) {
	defer recovery.Recover()
	c.Next()
}

