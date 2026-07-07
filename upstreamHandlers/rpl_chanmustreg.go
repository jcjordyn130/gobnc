package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle477(b Router, msg ircmsg.Message) error {
	// Sanity check
	if len(msg.Params) < 2 {
		log.Debug().Msgf("[upstream %s] Ignoring RPL 477 due to invalid paramaters", b.GetUpstreamConn().Server)
		return nil
	}

	channel := msg.Params[1]
	log.Debug().Msgf("[upstream %s] Cannot join channel %s due to being unregistered", b.GetUpstreamConn().Server, channel)

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
