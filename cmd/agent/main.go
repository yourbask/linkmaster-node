package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"linkmaster-node/internal/config"
	"linkmaster-node/internal/heartbeat"
	"linkmaster-node/internal/recovery"
	"linkmaster-node/internal/server"

	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger, err := initLogger(cfg)
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("节点服务启动", zap.String("version", "1.0.0"))

	// 初始化错误恢复
	recovery.Init()

	// 如果配置中没有节点信息，先发送一次心跳获取节点信息
	if cfg.Node.ID == 0 || cfg.Node.IP == "" {
		logger.Info("节点信息未配置，发送心跳获取节点信息")
		if err := heartbeat.RegisterNode(cfg); err != nil {
			logger.Warn("注册节点失败，将在心跳时重试", zap.Error(err))
		} else {
			logger.Info("节点信息已获取并保存",
				zap.Uint("node_id", cfg.Node.ID),
				zap.String("node_ip", cfg.Node.IP),
				zap.String("location", fmt.Sprintf("%s/%s/%s", cfg.Node.Country, cfg.Node.Province, cfg.Node.City)))
		}
	}

	// 启动心跳上报
	heartbeatReporter := heartbeat.NewReporter(cfg)
	go heartbeatReporter.Start(context.Background())

	// 启动HTTP服务器
	httpServer := server.NewHTTPServer(cfg)
	go func() {
		if err := httpServer.Start(); err != nil {
			logger.Fatal("HTTP服务器启动失败", zap.Error(err))
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("收到停止信号，正在关闭服务...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpServer.Shutdown(ctx)
	heartbeatReporter.Stop()

	logger.Info("服务已关闭")
}

func initLogger(cfg *config.Config) (*zap.Logger, error) {
	if cfg.Debug {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}

