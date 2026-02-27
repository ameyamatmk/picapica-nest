package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/ameyamatmk/picapica-nest/internal/logging"
	"github.com/ameyamatmk/picapica-nest/internal/provider"
	isession "github.com/ameyamatmk/picapica-nest/internal/session"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
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

func cmdServe() error {
	// 1. Config ロード
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	// 2. Provider 作成（Decorator chain: PromptRewrite → Logging → Inner）
	inner, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("provider creation error: %w", err)
	}
	usageLogPath := filepath.Join(cfg.WorkspacePath(), "usage.jsonl")
	loggingProvider := provider.NewLoggingProvider(inner, usageLogPath)
	llmProvider := provider.NewPromptRewriteProvider(loggingProvider)
	fmt.Printf("Provider chain: PromptRewrite → Logging(%s) → %T\n", usageLogPath, inner)

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
	channelManager, err := channels.NewManager(cfg, channelBus)
	if err != nil {
		return fmt.Errorf("channel manager creation error: %w", err)
	}
	agentLoop.SetChannelManager(channelManager)

	enabledChannels := channelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("Channels enabled: %s\n", enabledChannels)
	} else {
		fmt.Println("Warning: No channels enabled")
	}

	// 7. Health server 起動
	healthServer := health.NewServer(cfg.Gateway.Host, cfg.Gateway.Port)

	fmt.Printf("Gateway starting on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ConversationLogger のブリッジを起動（channelBus ↔ agentBus）
	convLogger.Run(ctx)

	// 8. 全チャンネル起動
	if err := channelManager.StartAll(ctx); err != nil {
		fmt.Printf("Error starting channels: %v\n", err)
	}

	// Health server をバックグラウンドで起動
	go func() {
		if err := healthServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	// Agent Loop をバックグラウンドで起動
	go agentLoop.Run(ctx)

	// IdleMonitor を起動（セッション idle timeout: 30分、チェック間隔: 1分）
	sessionsDirs := isession.SessionsDirsFromConfig(cfg)
	idleTimeout := 30 * time.Minute
	idleMonitor := isession.NewIdleMonitor(sessionsDirs, idleTimeout, 1*time.Minute)
	idleMonitor.Start(ctx)
	fmt.Printf("IdleMonitor started (timeout=%v, dirs=%v)\n", idleTimeout, sessionsDirs)

	fmt.Println("picapica-nest started. Press Ctrl+C to stop.")

	// 9. シグナルハンドリング + graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nShutting down...")
	cancel()
	idleMonitor.Stop()
	healthServer.Stop(context.Background())
	agentLoop.Stop()
	channelManager.StopAll(ctx)
	fmt.Println("picapica-nest stopped.")

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
		case "distill":
			err = cmdDistill(os.Args[2:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: picapica-nest [serve|distill]\n", os.Args[1])
			os.Exit(1)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
