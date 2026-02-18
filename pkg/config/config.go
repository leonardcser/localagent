package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type WebChatConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Config struct {
	Agents     AgentsConfig    `json:"agents"`
	Provider   ProviderConfig  `json:"provider"`
	Gateway    GatewayConfig   `json:"gateway"`
	Tools      ToolsConfig     `json:"tools"`
	Heartbeat  HeartbeatConfig `json:"heartbeat"`
	WebChat    WebChatConfig   `json:"webchat"`
	WebEnabled bool            `json:"web_enabled"`
	mu         sync.RWMutex
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

type AgentDefaults struct {
	Workspace         string  `json:"workspace"`
	Model             string  `json:"model"`
	MaxTokens         int     `json:"max_tokens"`
	Temperature       float64 `json:"temperature"`
	MaxToolIterations int     `json:"max_tool_iterations"`
}

type ProviderConfig struct {
	APIKeyEnv string `json:"api_key_env"`
	APIBase   string `json:"api_base"`
	Proxy     string `json:"proxy,omitempty"`
}

func (p ProviderConfig) ResolveAPIKey() string {
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}

type HeartbeatConfig struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval"` // minutes, min 5
}

type GatewayConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type BraveConfig struct {
	Enabled    bool   `json:"enabled"`
	APIKeyEnv  string `json:"api_key_env"`
	MaxResults int    `json:"max_results"`
}

func (b BraveConfig) ResolveAPIKey() string {
	if b.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(b.APIKeyEnv)
}

type DuckDuckGoConfig struct {
	Enabled    bool `json:"enabled"`
	MaxResults int  `json:"max_results"`
}

type WebToolsConfig struct {
	Brave      BraveConfig      `json:"brave"`
	DuckDuckGo DuckDuckGoConfig `json:"duckduckgo"`
}

type PDFConfig struct {
	URL       string `json:"url"`
	APIKeyEnv string `json:"api_key_env"`
}

func (p PDFConfig) ResolveAPIKey() string {
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}

type STTConfig struct {
	URL       string `json:"url"`
	APIKeyEnv string `json:"api_key_env"`
}

func (s STTConfig) ResolveAPIKey() string {
	if s.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(s.APIKeyEnv)
}

type ImageConfig struct {
	URL       string `json:"url"`
	APIKeyEnv string `json:"api_key_env"`
}

func (i ImageConfig) ResolveAPIKey() string {
	if i.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(i.APIKeyEnv)
}

type CronToolsConfig struct {
	ExecTimeoutMinutes int `json:"exec_timeout_minutes"`
}

type ToolsConfig struct {
	Web   WebToolsConfig `json:"web"`
	PDF   PDFConfig      `json:"pdf"`
	STT   STTConfig      `json:"stt"`
	Image ImageConfig    `json:"image"`
	Cron  CronToolsConfig `json:"cron"`
}

func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:         "~/.localagent/workspace",
				Model:             "llama3.2:latest",
				MaxTokens:         8192,
				Temperature:       0.7,
				MaxToolIterations: 20,
			},
		},
		Provider: ProviderConfig{
			APIBase: "http://localhost:11434/v1",
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18790,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Brave: BraveConfig{
					Enabled:    false,
					MaxResults: 5,
				},
				DuckDuckGo: DuckDuckGoConfig{
					Enabled:    false,
					MaxResults: 5,
				},
			},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:  true,
			Interval: 30,
		},
		WebChat: WebChatConfig{
			Host: "0.0.0.0",
			Port: 18791,
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file required: %w", err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) WorkspacePath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return expandHome(c.Agents.Defaults.Workspace)
}

func (c *Config) DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".localagent")
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("LOCALAGENT_WORKSPACE"); v != "" {
		cfg.Agents.Defaults.Workspace = v
	}
	if v := os.Getenv("LOCALAGENT_MODEL"); v != "" {
		cfg.Agents.Defaults.Model = v
	}
	if v := os.Getenv("LOCALAGENT_API_BASE"); v != "" {
		cfg.Provider.APIBase = v
	}
	if v := os.Getenv("LOCALAGENT_HEARTBEAT_ENABLED"); v == "false" || v == "0" {
		cfg.Heartbeat.Enabled = false
	}
	if v := os.Getenv("LOCALAGENT_HEALTH_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil {
			cfg.Gateway.Port = port
		}
	}
	if v := os.Getenv("LOCALAGENT_WEB_ENABLED"); v == "true" || v == "1" {
		cfg.WebEnabled = true
	}
}
