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
	"github.com/ameyamatmk/picapica-nest/internal/binding"
	"github.com/ameyamatmk/picapica-nest/internal/channellabel"
	"github.com/ameyamatmk/picapica-nest/internal/console"
	"github.com/ameyamatmk/picapica-nest/internal/logging"
	"github.com/ameyamatmk/picapica-nest/internal/pricing"
	"github.com/ameyamatmk/picapica-nest/internal/provider"
	isession "github.com/ameyamatmk/picapica-nest/internal/session"
	"github.com/ameyamatmk/picapica-nest/internal/slash"
	itools "github.com/ameyamatmk/picapica-nest/internal/tools"
	"github.com/bwmarrin/discordgo"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	_ "github.com/sipeed/picoclaw/pkg/channels/discord"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/health"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/tools"
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

// bindingStorePath は Binding 永続化ファイルのパスを返す。
func bindingStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picapica-nest", "bindings.json")
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

	// 7.5. Binding 復元（永続化されたチャンネル→Agent 割り当てを復元）
	bindingStore, err := binding.LoadOrNew(bindingStorePath())
	if err != nil {
		slog.Error("failed to load binding store", "error", err)
		bindingStore, _ = binding.LoadOrNew(filepath.Join(os.TempDir(), "picapica_bindings_fallback.json"))
	}
	restoreBindings(bindingStore, cfg, agentLoop, llmProvider)

	// 7.6. Claude Code CLI 委譲ツール登録
	// restoreBindings の後に登録することで、組み込みツール（web_fetch 等）を上書きする
	customTools := itools.NewClaudeTools(os.TempDir())
	for _, t := range customTools {
		agentLoop.RegisterTool(t)
	}
	slog.Info("claude code delegation tools registered", "tools", []string{"claude_analyze_image", "web_search", "web_fetch"})

	// 8. 全チャンネル起動
	if err := channelManager.StartAll(ctx); err != nil {
		slog.Error("failed to start channels", "error", err)
	}

	// 8.5. Discord スラッシュコマンド設定
	setupSlashCommands(channelManager, cfg, agentLoop, bindingStore, llmProvider, customTools)

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
	if bindingStore != nil {
		consoleOpts = append(consoleOpts, console.WithBindingStore(bindingStore))
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

// restoreBindings は永続化された Binding を cfg と AgentRegistry に復元する。
func restoreBindings(store *binding.Store, cfg *config.Config, agentLoop *agent.AgentLoop, llmProvider providers.LLMProvider) {
	entries := store.List()
	if len(entries) == 0 {
		return
	}

	slog.Info("restoring bindings", "count", len(entries))

	for _, entry := range entries {
		agentID := entry.AgentID

		// Agent が未登録なら動的作成
		if _, ok := agentLoop.Registry().GetAgent(agentID); !ok {
			agentCfg := &config.AgentConfig{
				ID:        agentID,
				Workspace: filepath.Join(cfg.Agents.Defaults.Workspace, "agents", agentID),
			}
			instance := agent.NewAgentInstance(agentCfg, &cfg.Agents.Defaults, cfg, llmProvider)
			if err := agentLoop.Registry().AddAgent(instance); err != nil {
				slog.Error("failed to restore agent", "agent_id", agentID, "error", err)
				continue
			}
			agentLoop.RegisterToolsForAgent(agentID)
			slog.Info("dynamic agent restored", "agent_id", agentID, "workspace", instance.Workspace)
		}
	}

	// cfg.Bindings に追加
	cfg.Bindings = append(cfg.Bindings, store.ToBindings()...)
	slog.Info("bindings restored to config", "total_bindings", len(cfg.Bindings))
}

// sessionProvider は Discord Session を返すインターフェース。
type sessionProvider interface {
	Session() *discordgo.Session
}

// setupSlashCommands は Discord チャンネルからセッションを取得しスラッシュコマンドを設定する。
func setupSlashCommands(
	channelManager *channels.Manager,
	cfg *config.Config,
	agentLoop *agent.AgentLoop,
	bindingStore *binding.Store,
	llmProvider providers.LLMProvider,
	customTools []tools.Tool,
) {
	ch, ok := channelManager.GetChannel("discord")
	if !ok {
		return
	}

	sp, ok := ch.(sessionProvider)
	if !ok {
		slog.Warn("discord channel does not expose Session()")
		return
	}

	sess := sp.Session()
	handler := slash.NewHandler(sess, cfg, agentLoop, bindingStore, llmProvider, customTools)
	sess.AddHandler(handler.HandleInteraction)

	slog.Info("slash commands handler registered", "component", "slash")
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
			slog.Error("unknown command", "command", os.Args[1])
			fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: picapica-nest [serve|hindsight]\n", os.Args[1])
			os.Exit(1)
		}
	}

	if err != nil {
		slog.Error("fatal error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
