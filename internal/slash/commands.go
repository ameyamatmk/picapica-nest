package slash

import "github.com/bwmarrin/discordgo"

// commands は登録するスラッシュコマンドの定義。
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "bind",
		Description: "このチャンネルに Agent を割り当てる",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "agent_id",
				Description: "割り当てる Agent の ID（未存在なら新規作成）",
				Required:    true,
			},
		},
	},
	{
		Name:        "unbind",
		Description: "このチャンネルの Agent 割り当てを解除する",
	},
	{
		Name:        "soul",
		Description: "Agent の追加指示を表示・編集する",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "view",
				Description: "現在の追加指示を表示する",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "edit",
				Description: "追加指示を編集する（モーダルが開きます）",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "reset",
				Description: "追加指示を削除してデフォルトに戻す",
			},
		},
	},
	{
		Name:        "status",
		Description: "このチャンネルの Agent 情報を表示する",
	},
}
