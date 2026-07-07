package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandleKICK(b Router, msg ircmsg.Message) error {
	// XXX: this *might* be safe, idk if the library checks for valid commands for us
	channel := msg.Params[0]
	target := msg.Params[1]

	// hoeh???
	if !b.IsJoined(channel) {
		log.Debug().Msgf("[upstream %s] KICK received for channel we're not even in??? %s", b.GetUpstreamConn().Server, channel)
		return nil
	}

	log.Debug().Msgf("[upstream %s] Processing KICK for user %s in %s", b.GetUpstreamConn().Server, target, channel)

	if target == b.GetUpstreamConn().CurrentNick() {
		// Remove channel from our internal channel list
		b.DeleteChannel(channel)
	} else {
		// Update user list
		b.RemoveUserFromChannel(channel, target)
	}

	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}
