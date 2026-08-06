package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Schedule ScheduleConfig `yaml:"schedule"`
	Fetcher  FetcherConfig  `yaml:"fetcher"`
	Feeds    []FeedConfig   `yaml:"feeds"`
	Filter   FilterConfig   `yaml:"filter"`
	ClientFilter ClientFilterConfig `yaml:"client_filter"`
	LLM      LLMConfig      `yaml:"llm"`
	Profile  string         `yaml:"profile"`
	Notify   NotifyConfig   `yaml:"notify"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type ScheduleConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// FetcherConfig 浏览器采集配置。CDP 模式：接管用户已运行的真实 Chrome，
// 浏览器需以 --remote-debugging-port 启动（见 README 的快捷方式说明）。
type FetcherConfig struct {
	CDPEndpoint string `yaml:"cdp_endpoint"`
	// PagesPerFeed 每个源翻页数，1=只抓第一页，上限 5（越多越容易被反爬盯上）
	PagesPerFeed int `yaml:"pages_per_feed"`
}

type FeedConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type FilterConfig struct {
	IncludeKeywords []string `yaml:"include_keywords"`
	ExcludeKeywords []string `yaml:"exclude_keywords"`
	MinBudgetUSD    int      `yaml:"min_budget_usd"`
}

// ClientFilterConfig 客户质量粗筛。StaleDays 为帖子超期天数（好单 48h 内必招满）。
type ClientFilterConfig struct {
	StaleDays int `yaml:"stale_days"`
}

type LLMConfig struct {
	BaseURL string        `yaml:"base_url"`
	APIKey  string        `yaml:"api_key"`
	Model   string        `yaml:"model"`
	Timeout time.Duration `yaml:"timeout"`
}

type NotifyConfig struct {
	Threshold int            `yaml:"threshold"`
	Telegram  TelegramConfig `yaml:"telegram"`
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// Load 读取 yaml 配置并用环境变量覆盖密钥类字段，环境变量优先级更高。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if v := os.Getenv("SCOUT_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("SCOUT_TG_TOKEN"); v != "" {
		cfg.Notify.Telegram.BotToken = v
	}
	if v := os.Getenv("SCOUT_TG_CHAT_ID"); v != "" {
		cfg.Notify.Telegram.ChatID = v
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Addr == "" {
		return errors.New("config: server.addr is required")
	}
	if c.Database.Path == "" {
		return errors.New("config: database.path is required")
	}
	// 频率下限：防高频抓取触发反爬
	if c.Schedule.Interval < 10*time.Minute {
		return errors.New("config: schedule.interval must be >= 10m")
	}
	if len(c.Feeds) == 0 {
		return errors.New("config: at least one feed is required")
	}
	for i, f := range c.Feeds {
		if f.URL == "" {
			return fmt.Errorf("config: feeds[%d].url is required", i)
		}
	}
	if c.Notify.Threshold < 0 || c.Notify.Threshold > 100 {
		return errors.New("config: notify.threshold must be between 0 and 100")
	}
	if c.Fetcher.PagesPerFeed < 0 || c.Fetcher.PagesPerFeed > 5 {
		return errors.New("config: fetcher.pages_per_feed must be between 0 and 5")
	}
	return nil
}
