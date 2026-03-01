package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/applog"
	"github.com/ameyamatmk/picapica-nest/internal/channellabel"
	"github.com/ameyamatmk/picapica-nest/internal/console"
	"github.com/ameyamatmk/picapica-nest/internal/logging"
	"github.com/ameyamatmk/picapica-nest/internal/pricing"
	"github.com/ameyamatmk/picapica-nest/internal/provider"
	isession "github.com/ameyamatmk/picapica-nest/internal/session"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	_ "github.com/sipeed/picoclaw/pkg/channels/discord"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/health"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// configPath は picapica-nest 専用の設定ファイルパスを返す。
// PicoClaw のデフォルト (~/.picoclaw/config.json) とは分離している。
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picapica-nest", "config.json")
}

// pricingConfigPath は pricing 設定ファイルのパスを返す。
func pricingConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picapica-nest", "pricing.json")
}

func cmdServe() error {
	// 1. Config ロード
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	// ロガー初期化
	logCloser, err := applog.Setup(cfg.WorkspacePath())
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer logCloser.Close()

	// 2. Provider 作成（Decorator chain: PromptRewrite → Logging → Inner）
	inner, _, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("provider creation error: %w", err)
	}
	usageLogPath := filepath.Join(cfg.WorkspacePath(), "usage.jsonl")
	loggingProvider := provider.NewLoggingProvider(inner, usageLogPath)
	llmProvider := provider.NewPromptRewriteProvider(loggingProvider, cfg.WorkspacePath())
	slog.Info("provider chain configured", "chain", fmt.Sprintf("PromptRewrite → Logging → %T", inner), "usage_log", usageLogPath)

	// 3. Message Bus 作成（Dual Bus + Bridge パターン）
	// channelBus: Channel 側（Channel が PublishInbound / SubscribeOutbound する先）
	// agentBus:   AgentLoop 側（AgentLoop が ConsumeInbound / PublishOutbound する先）
	// ConversationLogger が両者をブリッジし、通過するメッセージをログに記録する
	channelBus := bus.NewMessageBus()
	agentBus := bus.NewMessageBus()

	// 4. ConversationLogger 作成（channelBus ↔ agentBus のブリッジ）
	logBasePath := filepath.Join(cfg.WorkspacePath(), "logs")
	convLogger := logging.NewConversationLogger(logBasePath, channelBus, agentBus)

	// 5. Agent Loop 作成（agentBus を使用）
	agentLoop := agent.NewAgentLoop(cfg, agentBus, llmProvider)

	// 6. Channel Manager 作成（channelBus を使用）
	channelManager, err := channels.NewManager(cfg, channelBus, nil)
	if err != nil {
		return fmt.Errorf("channel manager creation error: %w", err)
	}
	agentLoop.SetChannelManager(channelManager)

	enabledChannels := channelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		slog.Info("channels enabled", "channels", enabledChannels)
	} else {
		slog.Warn("no channels enabled")
	}

	// 7. Health server 起動
	healthServer := health.NewServer(cfg.Gateway.Host, cfg.Gateway.Port)

	slog.Info("gateway starting", "host", cfg.Gateway.Host, "port", cfg.Gateway.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ConversationLogger のブリッジを起動（channelBus ↔ agentBus）
	convLogger.Run(ctx)

	// 8. 全チャンネル起動
	if err := channelManager.StartAll(ctx); err != nil {
		slog.Error("failed to start channels", "error", err)
	}

	// Health server をバックグラウンドで起動
	go func() {
		if err := healthServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("health server error", "error", err)
		}
	}()

	// チャンネルラベル解決の初期化
	labelStorePath := filepath.Join(cfg.WorkspacePath(), "channel_labels.json")
	labelStore, err := channellabel.NewStore(labelStorePath)
	if err != nil {
		slog.Warn("failed to load channel label store, starting with empty cache",
			"error", err, "path", labelStorePath)
		labelStore, _ = channellabel.NewStore(filepath.Join(os.TempDir(), "channel_labels_fallback.json"))
	}

	var labelResolver *channellabel.Resolver
	if cfg.Channels.Discord.Enabled && cfg.Channels.Discord.Token != "" {
		labelResolver = channellabel.NewResolver(cfg.Channels.Discord.Token, labelStore)
		slog.Info("channel label resolver configured", "component", "channellabel")
	}

	// Pricer 初期化（pricing.json が無い場合もエラーにならない）
	pricer, err := pricing.NewPricer(pricingConfigPath())
	if err != nil {
		slog.Warn("failed to load pricing config, cost display disabled", "error", err)
	}

	// Console server をバックグラウンドで起動
	var consoleOpts []console.ServerOption
	if labelResolver != nil {
		consoleOpts = append(consoleOpts, console.WithResolver(labelResolver))
	}
	if pricer != nil {
		consoleOpts = append(consoleOpts, console.WithPricer(pricer))
	}
	consoleServer := console.NewServer(cfg.WorkspacePath(), consoleOpts...)
	go func() {
		if err := consoleServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("console server error", "error", err)
		}
	}()

	// Agent Loop をバックグラウンドで起動
	go agentLoop.Run(ctx)

	// IdleMonitor を起動
	sessionsDirs := isession.SessionsDirsFromConfig(cfg)
	idleTimeoutMin := cfg.Agents.Defaults.IdleTimeoutMinutes
	if idleTimeoutMin <= 0 {
		idleTimeoutMin = 30
	}
	idleTimeout := time.Duration(idleTimeoutMin) * time.Minute
	idleMonitor := isession.NewIdleMonitor(sessionsDirs, idleTimeout, 1*time.Minute)
	idleMonitor.Start(ctx)
	slog.Info("idle monitor started", "component", "idle-monitor", "timeout", idleTimeout, "dirs", sessionsDirs)

	slog.Info("picapica-nest started")

	// 9. シグナルハンドリング + graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	slog.Info("shutting down")
	cancel()
	idleMonitor.Stop()
	consoleServer.Stop(context.Background())
	healthServer.Stop(context.Background())
	agentLoop.Stop()
	channelManager.StopAll(ctx)
	slog.Info("picapica-nest stopped")

	return nil
}

func main() {
	var err error

	if len(os.Args) < 2 {
		// 引数なし → serve（後方互換）
		err = cmdServe()
	} else {
		switch os.Args[1] {
		case "serve":
			err = cmdServe()
		case "hindsight":
			err = cmdHindsight(os.Args[2:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: picapica-nest [serve|hindsight]\n", os.Args[1])
			os.Exit(1)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
