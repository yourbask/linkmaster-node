package config

import (
	"fmt"
	"os"
	"path/filepath"

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

	// 节点信息（通过心跳获取并持久化）
	Node struct {
		ID       uint   `yaml:"id"`       // 节点ID
		IP       string `yaml:"ip"`       // 节点外网IP
		Country  string `yaml:"country"`  // 国家
		Province string `yaml:"province"` // 省份
		City     string `yaml:"city"`     // 城市
		ISP      string `yaml:"isp"`      // ISP
	} `yaml:"node"`
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

// Save 保存配置到文件
func (c *Config) Save() error {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	return configPath
}

