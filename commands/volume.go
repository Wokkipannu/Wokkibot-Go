package commands

import (
	"context"
	"wokkibot/wokkibot"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgolink/v3/lavalink"
)

var volumeCommand = discord.SlashCommandCreate{
	Name:        "volume",
	Description: "Set the volume",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "volume",
			Description: "The volume to set",
			Required:    true,
		},
	},
}

func HandleVolume(b *wokkibot.Wokkibot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		data := e.SlashCommandInteractionData()
		volume := data.Int("volume")

		player := b.Lavalink.ExistingPlayer(*e.GuildID())
		if player == nil {
			return e.CreateMessage(discord.NewMessageCreateBuilder().SetContent("No player found").Build())
		}

		if err := player.Update(context.TODO(), lavalink.WithVolume(volume)); err != nil {
			return e.CreateMessage(discord.NewMessageCreateBuilder().SetContent("Failed to set volume").Build())
		}

		return e.CreateMessage(discord.NewMessageCreateBuilder().SetContentf("Volume set to %d", volume).Build())
	}
}
