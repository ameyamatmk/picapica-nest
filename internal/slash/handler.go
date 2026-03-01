// Package slash は Discord スラッシュコマンドのハンドラを提供する。
package slash

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"

	"github.com/ameyamatmk/picapica-nest/internal/binding"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/routing"
)

// Handler は Discord スラッシュコマンドを処理する。
type Handler struct {
	session      *discordgo.Session
	cfg          *config.Config
	agentLoop    *agent.AgentLoop
	bindingStore *binding.Store
	provider     providers.LLMProvider
	mu           sync.Mutex
	registeredIn map[string]bool // guildID → 登録済みフラグ
}

// NewHandler は新しい Handler を作成する。
func NewHandler(
	session *discordgo.Session,
	cfg *config.Config,
	agentLoop *agent.AgentLoop,
	bindingStore *binding.Store,
	provider providers.LLMProvider,
) *Handler {
	return &Handler{
		session:      session,
		cfg:          cfg,
		agentLoop:    agentLoop,
		bindingStore: bindingStore,
		provider:     provider,
		registeredIn: make(map[string]bool),
	}
}

// RegisterCommands は Guild コマンドとして全コマンドを登録する。
// Session.Open() 後に呼ぶこと。
func (h *Handler) RegisterCommands(guildID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.registeredIn[guildID] {
		return nil
	}

	appID := h.session.State.User.ID
	for _, cmd := range commands {
		_, err := h.session.ApplicationCommandCreate(appID, guildID, cmd)
		if err != nil {
			return fmt.Errorf("slash: register command %q in guild %s: %w", cmd.Name, guildID, err)
		}
		slog.Info("slash command registered",
			"command", cmd.Name,
			"guild_id", guildID,
		)
	}

	h.registeredIn[guildID] = true
	return nil
}

// HandleInteraction は InteractionCreate イベントのハンドラ。
// discordgo.Session.AddHandler() で登録する。
func (h *Handler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()

		// Guild コマンドの自動登録（まだ未登録の Guild の場合）
		if i.GuildID != "" {
			if err := h.RegisterCommands(i.GuildID); err != nil {
				slog.Error("slash: auto-register commands failed", "guild_id", i.GuildID, "error", err)
			}
		}

		switch data.Name {
		case "bind":
			h.handleBind(s, i)
		case "unbind":
			h.handleUnbind(s, i)
		case "soul":
			h.handleSoul(s, i)
		case "status":
			h.handleStatus(s, i)
		default:
			respondEphemeral(s, i, "Unknown command: "+data.Name)
		}

	case discordgo.InteractionModalSubmit:
		data := i.ModalSubmitData()
		if strings.HasPrefix(data.CustomID, "soul_edit:") {
			h.handleSoulEditSubmit(s, i)
		}
	}
}

func (h *Handler) handleBind(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	opt := data.GetOption("agent_id")
	if opt == nil {
		respondEphemeral(s, i, "agent_id is required")
		return
	}
	agentID := strings.TrimSpace(opt.StringValue())
	if agentID == "" {
		respondEphemeral(s, i, "agent_id must not be empty")
		return
	}

	channelID := i.ChannelID

	h.mu.Lock()
	defer h.mu.Unlock()

	// 既存 Binding チェック
	normalizedID := routing.NormalizeAgentID(agentID)

	// 既存 binding があれば付け替え
	var oldAgentID string
	if existing := h.bindingStore.FindByChannel(channelID); existing != nil {
		if existing.AgentID == normalizedID {
			respondEphemeral(s, i, fmt.Sprintf("This channel is already bound to agent `%s`.", normalizedID))
			return
		}
		oldAgentID = existing.AgentID
		h.bindingStore.Remove(channelID)
		h.removeConfigBinding(channelID)
		slog.Info("slash: rebinding channel", "old_agent", oldAgentID, "new_agent", normalizedID, "channel_id", channelID)
	}

	// Agent が未登録なら新規作成
	if _, ok := h.agentLoop.Registry().GetAgent(normalizedID); !ok {
		if err := h.createAgent(normalizedID); err != nil {
			respondEphemeral(s, i, fmt.Sprintf("Failed to create agent `%s`: %s", normalizedID, err))
			return
		}
	}

	// Binding 追加
	if err := h.bindingStore.Add(normalizedID, channelID); err != nil {
		respondEphemeral(s, i, fmt.Sprintf("Failed to add binding: %s", err))
		return
	}

	// cfg.Bindings に追加（新スライス作成でアトミック代入）
	h.appendConfigBinding(normalizedID, channelID)

	// 永続化
	if err := h.bindingStore.Save(); err != nil {
		slog.Error("slash: failed to save binding store", "error", err)
	}

	if oldAgentID != "" {
		respondEphemeral(s, i, fmt.Sprintf(
			"Agent switched: `%s` → `%s`\nMessages here will now be handled by `%s`.",
			oldAgentID, normalizedID, normalizedID,
		))
	} else {
		respondEphemeral(s, i, fmt.Sprintf(
			"Agent `%s` bound to this channel.\nMessages here will be handled by this agent.",
			normalizedID,
		))
	}
	slog.Info("slash: channel bound",
		"agent_id", normalizedID,
		"channel_id", channelID,
		"guild_id", i.GuildID,
	)
}

func (h *Handler) handleUnbind(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID

	h.mu.Lock()
	defer h.mu.Unlock()

	entry := h.bindingStore.FindByChannel(channelID)
	if entry == nil {
		respondEphemeral(s, i, "This channel has no agent binding.")
		return
	}

	agentID := entry.AgentID

	h.bindingStore.Remove(channelID)

	// cfg.Bindings から削除
	h.removeConfigBinding(channelID)

	if err := h.bindingStore.Save(); err != nil {
		slog.Error("slash: failed to save binding store", "error", err)
	}

	respondEphemeral(s, i, fmt.Sprintf(
		"Agent `%s` unbound from this channel.\nMessages will now be handled by the default agent.",
		agentID,
	))
	slog.Info("slash: channel unbound",
		"agent_id", agentID,
		"channel_id", channelID,
	)
}

const soulExtraFileName = "SOUL_EXTRA.md"
const soulSeparator = "\n\n---\n\n"

func (h *Handler) handleSoul(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		respondEphemeral(s, i, "Subcommand required: `view`, `edit`, or `reset`")
		return
	}

	sub := data.Options[0]
	channelID := i.ChannelID

	// チャンネルに紐付いた Agent を取得
	entry := h.bindingStore.FindByChannel(channelID)
	if entry == nil {
		respondEphemeral(s, i, "This channel has no agent binding. Use `/bind` first.")
		return
	}

	instance, ok := h.agentLoop.Registry().GetAgent(entry.AgentID)
	if !ok {
		respondEphemeral(s, i, fmt.Sprintf("Agent `%s` not found in registry.", entry.AgentID))
		return
	}

	switch sub.Name {
	case "view":
		extraPath := filepath.Join(instance.Workspace, soulExtraFileName)
		content, err := os.ReadFile(extraPath)
		if err != nil || len(strings.TrimSpace(string(content))) == 0 {
			respondEphemeral(s, i, fmt.Sprintf("Agent `%s` has no extra instructions.", entry.AgentID))
			return
		}
		text := string(content)
		if len(text) > 1900 {
			text = text[:1900] + "\n... (truncated)"
		}
		respondEphemeral(s, i, fmt.Sprintf("**Extra instructions** for `%s`:\n```\n%s\n```", entry.AgentID, text))

	case "edit":
		// 現在の SOUL_EXTRA.md を読み込んで Modal にプリフィル
		extraPath := filepath.Join(instance.Workspace, soulExtraFileName)
		currentExtra := ""
		if data, err := os.ReadFile(extraPath); err == nil {
			currentExtra = string(data)
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "soul_edit:" + channelID,
				Title:    "追加指示の編集 (" + entry.AgentID + ")",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "soul_content",
								Label:       "追加指示",
								Style:       discordgo.TextInputParagraph,
								Placeholder: "このエージェントへの追加指示を入力...",
								Value:       currentExtra,
								Required:    true,
								MaxLength:   4000,
							},
						},
					},
				},
			},
		}); err != nil {
			slog.Error("slash: failed to show modal", "error", err)
		}

	case "reset":
		extraPath := filepath.Join(instance.Workspace, soulExtraFileName)
		os.Remove(extraPath)

		// SOUL.md をテンプレに復元
		if err := h.rebuildSoulMD(instance.Workspace, ""); err != nil {
			respondEphemeral(s, i, fmt.Sprintf("Failed to reset SOUL.md: %s", err))
			return
		}
		respondEphemeral(s, i, fmt.Sprintf("Agent `%s` extra instructions cleared. SOUL.md restored to default.", entry.AgentID))
		slog.Info("slash: SOUL.md reset", "agent_id", entry.AgentID)
	}
}

// handleSoulEditSubmit は Modal Submit を処理する。
func (h *Handler) handleSoulEditSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()

	// customID = "soul_edit:<channelID>"
	channelID := strings.TrimPrefix(data.CustomID, "soul_edit:")

	entry := h.bindingStore.FindByChannel(channelID)
	if entry == nil {
		respondEphemeral(s, i, "This channel has no agent binding.")
		return
	}

	instance, ok := h.agentLoop.Registry().GetAgent(entry.AgentID)
	if !ok {
		respondEphemeral(s, i, fmt.Sprintf("Agent `%s` not found.", entry.AgentID))
		return
	}

	// Modal から入力内容を取得
	var extraContent string
	for _, comp := range data.Components {
		row, ok := comp.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, rowComp := range row.Components {
			input, ok := rowComp.(*discordgo.TextInput)
			if ok && input.CustomID == "soul_content" {
				extraContent = input.Value
			}
		}
	}

	// SOUL_EXTRA.md に保存
	extraPath := filepath.Join(instance.Workspace, soulExtraFileName)
	if err := os.WriteFile(extraPath, []byte(extraContent), 0o644); err != nil {
		respondEphemeral(s, i, fmt.Sprintf("Failed to save extra instructions: %s", err))
		return
	}

	// SOUL.md を再生成（テンプレ + 追加指示）
	if err := h.rebuildSoulMD(instance.Workspace, extraContent); err != nil {
		respondEphemeral(s, i, fmt.Sprintf("Failed to rebuild SOUL.md: %s", err))
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Agent `%s` extra instructions updated.", entry.AgentID))
	slog.Info("slash: SOUL_EXTRA.md updated",
		"agent_id", entry.AgentID,
		"length", len(extraContent),
	)
}

// rebuildSoulMD はテンプレ SOUL.md + 追加指示を結合して Agent の SOUL.md に書き出す。
func (h *Handler) rebuildSoulMD(workspace, extra string) error {
	// テンプレートを読む
	templatePath := filepath.Join(h.cfg.WorkspacePath(), "templates", "SOUL.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		templatePath = filepath.Join(h.cfg.WorkspacePath(), "SOUL.md")
	}

	template, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	content := string(template)
	if strings.TrimSpace(extra) != "" {
		content += soulSeparator + extra
	}

	soulPath := filepath.Join(workspace, "SOUL.md")
	return os.WriteFile(soulPath, []byte(content), 0o644)
}

func (h *Handler) handleStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID

	entry := h.bindingStore.FindByChannel(channelID)
	if entry == nil {
		respondEphemeral(s, i, "This channel has no agent binding.\nMessages are handled by the default agent.")
		return
	}

	instance, ok := h.agentLoop.Registry().GetAgent(entry.AgentID)
	if !ok {
		respondEphemeral(s, i, fmt.Sprintf(
			"Agent `%s` is bound but not found in registry.\nThis may indicate a restart issue.",
			entry.AgentID,
		))
		return
	}

	msg := fmt.Sprintf(
		"**Agent Status**\n"+
			"- **ID**: `%s`\n"+
			"- **Name**: %s\n"+
			"- **Model**: `%s`\n"+
			"- **Workspace**: `%s`\n"+
			"- **Bound since**: %s",
		instance.ID,
		displayName(instance),
		instance.Model,
		instance.Workspace,
		entry.CreatedAt,
	)
	respondEphemeral(s, i, msg)
}

// createAgent は新しい Agent を動的に作成・登録する。
func (h *Handler) createAgent(agentID string) error {
	agentCfg := &config.AgentConfig{
		ID:        agentID,
		Workspace: filepath.Join(h.cfg.Agents.Defaults.Workspace, "agents", agentID),
	}
	defaults := &h.cfg.Agents.Defaults

	instance := agent.NewAgentInstance(agentCfg, defaults, h.cfg, h.provider)

	// デフォルト SOUL.md をコピー
	if err := h.copySoulTemplate(instance.Workspace); err != nil {
		slog.Warn("slash: failed to copy SOUL.md template", "error", err)
	}

	if err := h.agentLoop.Registry().AddAgent(instance); err != nil {
		return err
	}

	h.agentLoop.RegisterToolsForAgent(agentID)

	slog.Info("slash: dynamic agent created",
		"agent_id", agentID,
		"workspace", instance.Workspace,
		"model", instance.Model,
	)
	return nil
}

// copySoulTemplate はデフォルトワークスペースの SOUL.md を新ワークスペースにコピーする。
func (h *Handler) copySoulTemplate(workspace string) error {
	templatePath := filepath.Join(h.cfg.WorkspacePath(), "templates", "SOUL.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		// テンプレートが無ければデフォルトワークスペースの SOUL.md を使う
		templatePath = filepath.Join(h.cfg.WorkspacePath(), "SOUL.md")
	}

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	destPath := filepath.Join(workspace, "SOUL.md")
	if _, err := os.Stat(destPath); err == nil {
		return nil // 既に存在する場合はスキップ
	}

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return fmt.Errorf("mkdir workspace: %w", err)
	}

	return os.WriteFile(destPath, content, 0o644)
}

// appendConfigBinding は cfg.Bindings に新しい Binding を追加する。
// 新スライスを作成してアトミック代入する。
func (h *Handler) appendConfigBinding(agentID, channelID string) {
	newBinding := config.AgentBinding{
		AgentID: agentID,
		Match: config.BindingMatch{
			Channel:   "discord",
			AccountID: "*",
			Peer: &config.PeerMatch{
				Kind: "channel",
				ID:   channelID,
			},
		},
	}
	newBindings := make([]config.AgentBinding, len(h.cfg.Bindings)+1)
	copy(newBindings, h.cfg.Bindings)
	newBindings[len(newBindings)-1] = newBinding
	h.cfg.Bindings = newBindings
}

// removeConfigBinding は cfg.Bindings から指定チャンネルの Binding を削除する。
func (h *Handler) removeConfigBinding(channelID string) {
	var newBindings []config.AgentBinding
	for _, b := range h.cfg.Bindings {
		if b.Match.Peer != nil && b.Match.Peer.Kind == "channel" && b.Match.Peer.ID == channelID {
			continue
		}
		newBindings = append(newBindings, b)
	}
	h.cfg.Bindings = newBindings
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		slog.Error("slash: respond error", "error", err)
	}
}

func displayName(instance *agent.AgentInstance) string {
	if instance.Name != "" {
		return instance.Name
	}
	return instance.ID
}

// getSubOption は ApplicationCommandInteractionDataOption からサブオプションを名前で検索する。
func getSubOption(sub *discordgo.ApplicationCommandInteractionDataOption, name string) *discordgo.ApplicationCommandInteractionDataOption {
	for _, opt := range sub.Options {
		if opt.Name == name {
			return opt
		}
	}
	return nil
}
