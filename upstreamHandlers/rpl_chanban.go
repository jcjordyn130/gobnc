package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle474(b Router, msg ircmsg.Message) (bool, error) {
	// Sanity check
	if len(msg.Params) < 2 {
		log.Debug().Msgf("[upstream %s] Ignoring RPL 474 due to invalid paramaters", b.GetUpstreamConn().Config.Server)
		return false, nil
	}

	channel := msg.Params[1]
	log.Debug().Msgf("[upstream %s] Cannot join channel %s due to ban", b.GetUpstreamConn().Config.Server, channel)

	// Forward message to client
	b.BroadcastToClients(msg)

	return true, nil
}
