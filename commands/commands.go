package commands

import (
	"wokkibot/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/lukasl-dev/waterlink/v2"
)

type Command struct {
	Info *discordgo.ApplicationCommand
	Run  func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

var (
	Commands = []*discordgo.ApplicationCommand{
		// play.Info,
		// skip.Info,
		// volume.Info,
		// queue.Info,
		// seek.Info,
		// disconnect.Info,
		// Other commands
		roll.Info,
		friday.Info,
		pizza.Info,
		user.Info,
		flip.Info,
		quote.Info,
	}
	Handlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		// Music related commands
		// play.Info.Name:       play.Run,
		// skip.Info.Name:       skip.Run,
		// volume.Info.Name:     volume.Run,
		// queue.Info.Name:      queue.Run,
		// seek.Info.Name:       seek.Run,
		// disconnect.Info.Name: disconnect.Run,
		// Other commands
		roll.Info.Name:   roll.Run,
		friday.Info.Name: friday.Run,
		pizza.Info.Name:  pizza.Run,
		user.Info.Name:   user.Run,
		flip.Info.Name:   flip.Run,
		quote.Info.Name:  quote.Run,
	}
	Session             *discordgo.Session
	WaterlinkClient     *waterlink.Client
	WaterlinkConnection *waterlink.Connection
	SessionID           string
)

func ListenForEvents() {
	// for evt := range Conn.Events() {
	// 	switch evt.Type() {
	// 	case event.WebsocketClosed:
	// 		evt := evt.(server.WebsocketClosed)
	// 		log.Printf("Websocket connection to discord closed: %v", evt.Reason)
	// 		// continueTracks(evt.GuildID)
	// 	case event.TrackException:
	// 		evt := evt.(player.TrackException)
	// 		log.Printf("Exception occurred in an audio track: %v", evt.Error)
	// 		continueTracks(evt.GuildID)
	// 	case event.TrackStuck:
	// 		evt := evt.(player.TrackStuck)
	// 		log.Printf("Track %v was started, but no audio frames from it have arrived in a long time in guild %v", evt.TrackID, evt.GuildID)
	// 		continueTracks(evt.GuildID)
	// 	case event.TrackEnd:
	// 		evt := evt.(player.TrackEnd)
	// 		log.Printf("Track %v ended in guild %v", evt.TrackID, evt.GuildID)
	// 		continueTracks(evt.GuildID)
	// 	}
	// }
}

func continueTracks(guildId string) {
	if _, ok := utils.Queue[guildId]; ok {
		utils.Queue[guildId].Queue = utils.Queue[guildId].Queue[1:]
		BeginPlay(guildId, nil)
	}
}
