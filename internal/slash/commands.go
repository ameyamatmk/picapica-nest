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
		Description: "Agent の SOUL.md を表示・編集する",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "view",
				Description: "SOUL.md を表示する",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "edit",
				Description: "SOUL.md を上書きする",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "content",
						Description: "新しい SOUL.md の内容",
						Required:    true,
					},
				},
			},
		},
	},
	{
		Name:        "status",
		Description: "このチャンネルの Agent 情報を表示する",
	},
}
