package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandleQUIT(b Router, msg ircmsg.Message) error {
	// XXX: this *might* be safe, idk if the library checks for valid commands for us
	quittingNick := msg.Nick()
	log.Debug().Msgf("[upstream %s] Processing QUIT for user %s", b.GetUpstreamConn().Server, quittingNick)

	// If somehow we get an upsteam quit for ourselves
	if b.GetUpstreamConn() != nil && quittingNick == b.GetUpstreamConn().CurrentNick() {
		log.Debug().Msgf("[upstream %s] Got QUIT for ourselves! !!NOOP!!", b.GetUpstreamConn().Server)
		return nil
	}

	b.RemoveUserFromAllChannels(quittingNick)

	// Broadcast to clients to they can update their state
	b.BroadcastToClients(msg)

	return nil
}
