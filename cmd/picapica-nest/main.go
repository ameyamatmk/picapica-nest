package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

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

func run() error {
	// 1. Config ロード
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	// 2. Provider 作成
	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("provider creation error: %w", err)
	}

	// 解決済みの model ID を反映
	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	// 3. Message Bus 作成
	msgBus := bus.NewMessageBus()

	// 4. Agent Loop 作成
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	// 5. Channel Manager 作成
	channelManager, err := channels.NewManager(cfg, msgBus)
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

	// 6. Health server 起動
	healthServer := health.NewServer(cfg.Gateway.Host, cfg.Gateway.Port)

	fmt.Printf("Gateway starting on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 7. 全チャンネル起動
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

	fmt.Println("picapica-nest started. Press Ctrl+C to stop.")

	// 8. シグナルハンドリング + graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nShutting down...")
	if cp, ok := provider.(providers.StatefulProvider); ok {
		cp.Close()
	}
	cancel()
	healthServer.Stop(context.Background())
	agentLoop.Stop()
	channelManager.StopAll(ctx)
	fmt.Println("picapica-nest stopped.")

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
