package commands

import (
	"log"
	"time"
	"wokkibot/utils"

	"github.com/bwmarrin/discordgo"
)

var quote = Command{
	Info: &discordgo.ApplicationCommand{
		Name: "Quote",
		Type: discordgo.MessageApplicationCommand,
	},
	Run: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		msg := i.ApplicationCommandData().Resolved.Messages[i.ApplicationCommandData().TargetID]

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Author: &discordgo.MessageEmbedAuthor{
							Name:    "Quote from " + msg.Author.Username,
							IconURL: msg.Author.AvatarURL(""),
						},
						Description: msg.Content,
						Timestamp:   msg.Timestamp.Format(time.RFC3339),
						Color:       msg.Author.AccentColor,
					},
				},
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label: "Go to message",
								Style: discordgo.LinkButton,
								URL:   "https://discord.com/channels/" + i.GuildID + "/" + msg.ChannelID + "/" + msg.ID,
							},
						},
					},
				},
			},
		})
		if err != nil {
			log.Println(err)
			utils.InteractionRespondMessage(s, i, "Failed to quote")
			return
		}
	},
}
