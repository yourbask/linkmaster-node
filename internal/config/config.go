package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Backend struct {
		URL string `yaml:"url"`
	} `yaml:"backend"`

	Heartbeat struct {
		Interval int `yaml:"interval"` // 心跳间隔（秒）
	} `yaml:"heartbeat"`

	Debug bool `yaml:"debug"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	// 默认配置
	cfg.Server.Port = 2200
	cfg.Heartbeat.Interval = 60
	cfg.Debug = false

	// 从环境变量读取后端URL
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}
	cfg.Backend.URL = backendURL

	// 尝试从配置文件读取
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件失败: %w", err)
		}
	}

	return cfg, nil
}

