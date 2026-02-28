package channellabel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode"
)

// discordAPIBase は Discord API のベース URL。テストで差し替え可能。
var discordAPIBase = "https://discord.com/api/v10"

// discordChannel は Discord Get Channel API のレスポンスから必要なフィールドだけを抽出する。
type discordChannel struct {
	Name       string            `json:"name"`
	Type       int               `json:"type"`
	Recipients []discordRecipient `json:"recipients"`
}

// discordRecipient は DM チャンネルの受信者情報。
type discordRecipient struct {
	Username string `json:"username"`
}

// Resolver は Discord API でチャンネル名を解決する。
type Resolver struct {
	token  string
	store  *Store
	client *http.Client
}

// NewResolver は新しい Resolver を作成する。
func NewResolver(token string, store *Store) *Resolver {
	return &Resolver{
		token:  token,
		store:  store,
		client: &http.Client{},
	}
}

// Resolve はディレクトリ名からラベルを解決する。
// キャッシュにあればそれを返す。なければ Discord API で取得してキャッシュに保存する。
// 非 Discord ディレクトリや非数値 ID の場合はエラーを返す。
func (r *Resolver) Resolve(dirName string) (string, error) {
	// キャッシュチェック
	if label, ok := r.store.Get(dirName); ok {
		return label, nil
	}

	// ディレクトリ名から channel ID を抽出
	channelID, ok := extractChannelID(dirName)
	if !ok {
		return "", fmt.Errorf("not a resolvable directory name: %s", dirName)
	}

	// Discord API でチャンネル名を取得
	label, err := r.fetchChannelName(channelID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch channel name for %s: %w", channelID, err)
	}

	// キャッシュに保存
	if err := r.store.Set(dirName, label); err != nil {
		slog.Warn("failed to save channel label to cache",
			"component", "channellabel", "dirName", dirName, "error", err)
	}

	return label, nil
}

// extractChannelID はディレクトリ名から数値の Discord チャンネル ID を抽出する。
// "discord_1469682598732239006" → "1469682598732239006", true
// "discord_test-channel" → "", false（非数値）
func extractChannelID(dirName string) (string, bool) {
	idx := strings.Index(dirName, "_")
	if idx < 0 {
		return "", false
	}

	chatID := dirName[idx+1:]
	if chatID == "" {
		return "", false
	}

	// 数値のみかチェック
	for _, r := range chatID {
		if !unicode.IsDigit(r) {
			return "", false
		}
	}

	return chatID, true
}

// fetchChannelName は Discord API でチャンネル名を取得する。
func (r *Resolver) fetchChannelName(channelID string) (string, error) {
	url := fmt.Sprintf("%s/channels/%s", discordAPIBase, channelID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+r.token)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Discord API returned status %d", resp.StatusCode)
	}

	var ch discordChannel
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return "", err
	}

	// DM チャンネル（type: 1）
	if ch.Type == 1 && len(ch.Recipients) > 0 {
		return "DM: " + ch.Recipients[0].Username, nil
	}

	return ch.Name, nil
}
