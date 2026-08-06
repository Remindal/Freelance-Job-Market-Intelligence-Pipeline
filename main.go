package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/Remindal/scout/internal/config"
	"github.com/Remindal/scout/internal/desktop"
	"github.com/Remindal/scout/internal/fetcher"
	"github.com/Remindal/scout/internal/filter"
	"github.com/Remindal/scout/internal/llm"
	"github.com/Remindal/scout/internal/notify"
	"github.com/Remindal/scout/internal/pipeline"
	"github.com/Remindal/scout/internal/scheduler"
	"github.com/Remindal/scout/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.ico
var trayIcon []byte

// resolveExeDir 所有相对路径（config.yaml / scout.db）一律从 exe 所在目录推导，
// 不依赖 CWD，兼容双击 / 快捷方式 / 服务模式启动。
func resolveExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func resolvePath(exeDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(exeDir, p)
}

func main() {
	exeDir := resolveExeDir()

	configPath := os.Getenv("SCOUT_CONFIG")
	if configPath == "" {
		configPath = filepath.Join(exeDir, "configs", "config.yaml")
	}

	// 日志同时落 exe 同级文件，GUI 程序无控制台可见输出。
	// 注意文件必须放 MultiWriter 第一位：GUI 子系统下 stderr 写入会报错，
	// 而 MultiWriter 遇错即短路，文件在前才能保证落盘。
	logFile, err := os.OpenFile(filepath.Join(exeDir, "scout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	logWriter := io.Writer(os.Stderr)
	if err == nil {
		defer logFile.Close()
		logWriter = io.MultiWriter(logFile, os.Stderr)
	}
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("load config failed", "path", configPath, "err", err)
		os.Exit(1)
	}

	dbPath := resolvePath(exeDir, cfg.Database.Path)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		logger.Error("create data dir failed", "err", err)
		os.Exit(1)
	}
	st, err := store.NewSQLite(dbPath)
	if err != nil {
		logger.Error("init store failed", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	feeds := make([]fetcher.Feed, len(cfg.Feeds))
	for i, f := range cfg.Feeds {
		feeds[i] = fetcher.Feed{Name: f.Name, URL: f.URL}
	}
	pwFetcher := fetcher.NewPlaywright(feeds, cfg.Fetcher.CDPEndpoint, cfg.Fetcher.PagesPerFeed, logger)

	rules := filter.NewRules(cfg.Filter.IncludeKeywords, cfg.Filter.ExcludeKeywords, cfg.Filter.MinBudgetUSD)
	clientFilter := filter.NewClientFilter(cfg.ClientFilter.StaleDays)

	var scorer *filter.Scorer
	if cfg.LLM.APIKey != "" {
		llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Timeout)
		scorer = filter.NewScorer(llmClient, cfg.Profile, 3, logger)
	} else {
		logger.Warn("llm api_key empty, scoring disabled (jobs saved with score=0)")
	}

	var notifiers []notify.Notifier
	if cfg.Notify.Telegram.BotToken != "" && cfg.Notify.Telegram.ChatID != "" {
		notifiers = append(notifiers, notify.NewTelegram(
			cfg.Notify.Telegram.BotToken, cfg.Notify.Telegram.ChatID, ""))
	} else {
		logger.Warn("telegram not configured, notification disabled")
	}

	pl := pipeline.New(pwFetcher, st, rules, clientFilter, scorer, notifiers, cfg.Notify.Threshold, logger)
	sched := scheduler.New(cfg.Schedule.Interval, func(ctx context.Context) error {
		_, err := pl.RunOnce(ctx)
		return err
	}, logger)
	app := desktop.NewApp(st, pl, sched, cfg.Notify.Threshold, logger)

	err = wails.Run(&options.App{
		Title:     "Scout",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			go runTray(app)
		},
		OnShutdown: func(ctx context.Context) {
			app.Shutdown(ctx)
		},
		// 点 X 不退出，最小化到托盘；退出只走托盘菜单
		OnBeforeClose: func(ctx context.Context) bool {
			runtime.WindowHide(ctx)
			return true
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "9f2c4e7a-scout-single-instance",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				ctx := app.Context()
				if ctx != nil {
					runtime.Show(ctx)
					runtime.WindowUnminimise(ctx)
				}
			},
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		logger.Error("wails run failed", "err", err)
		os.Exit(1)
	}
}

// runTray 系统托盘：打开面板 / 立即抓取 / 退出。
func runTray(app *desktop.App) {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTooltip("Scout")

		// 每分钟刷新 tooltip 中的今日新单数
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if stats, err := app.GetStats(); err == nil {
					systray.SetTooltip(fmt.Sprintf("Scout · 今日新单 %d", stats.TodayNew))
				}
			}
		}()

		mOpen := systray.AddMenuItem("打开面板", "显示主窗口")
		mFetch := systray.AddMenuItem("立即抓取", "立即运行一轮采集")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "退出程序")

		mOpen.Click(func() {
			ctx := app.Context()
			if ctx != nil {
				runtime.Show(ctx)
				runtime.WindowUnminimise(ctx)
			}
		})
		mFetch.Click(func() {
			res, err := app.RunNow()
			if err != nil {
				app.LogInfo("manual fetch skipped: " + err.Error())
			} else {
				app.LogInfo("manual fetch: " + res.Message)
			}
		})
		mQuit.Click(func() {
			ctx := app.Context()
			if ctx != nil {
				runtime.Quit(ctx)
			}
			systray.Quit()
		})
	}, func() {})
}
