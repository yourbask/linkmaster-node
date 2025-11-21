package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"linkmaster-node/internal/config"
	"linkmaster-node/internal/handler"
	"linkmaster-node/internal/recovery"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type HTTPServer struct {
	ipv4Server *http.Server
	ipv6Server *http.Server
	logger     *zap.Logger
	wg         sync.WaitGroup
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

	// IPv4 服务器
	ipv4Server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Server.Port),
		Handler: router,
	}

	// IPv6 服务器
	ipv6Server := &http.Server{
		Addr:    fmt.Sprintf("[::]:%d", cfg.Server.Port),
		Handler: router,
	}

	logger, _ := zap.NewProduction()

	return &HTTPServer{
		ipv4Server: ipv4Server,
		ipv6Server: ipv6Server,
		logger:     logger,
	}
}

func (s *HTTPServer) Start() error {
	// 启动 IPv4 服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("HTTP服务器启动 (IPv4)", zap.String("addr", s.ipv4Server.Addr))
		if err := s.ipv4Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("IPv4服务器启动失败", zap.Error(err))
		}
	}()

	// 启动 IPv6 服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 检查系统是否支持 IPv6
		listener, err := net.Listen("tcp6", s.ipv6Server.Addr)
		if err != nil {
			s.logger.Warn("IPv6不支持或已禁用，跳过IPv6监听", zap.String("addr", s.ipv6Server.Addr), zap.Error(err))
			return
		}
		
		s.logger.Info("HTTP服务器启动 (IPv6)", zap.String("addr", s.ipv6Server.Addr))
		if err := s.ipv6Server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("IPv6服务器启动失败", zap.Error(err))
		}
		// Serve 返回后关闭监听器
		listener.Close()
	}()

	// 立即返回，服务器在后台运行
	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	var errs []error

	// 关闭 IPv4 服务器
	if s.ipv4Server != nil {
		if err := s.ipv4Server.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("关闭IPv4服务器失败: %w", err))
		} else {
			s.logger.Info("IPv4服务器已关闭")
		}
	}

	// 关闭 IPv6 服务器
	if s.ipv6Server != nil {
		if err := s.ipv6Server.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("关闭IPv6服务器失败: %w", err))
		} else {
			s.logger.Info("IPv6服务器已关闭")
		}
	}

	// 等待所有 goroutine 完成
	s.wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("关闭服务器时发生错误: %v", errs)
	}

	return nil
}

func recoveryMiddleware(c *gin.Context) {
	defer recovery.Recover()
	c.Next()
}

