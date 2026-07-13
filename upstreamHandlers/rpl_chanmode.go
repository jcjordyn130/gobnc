package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle324(b Router, msg ircmsg.Message) (bool, error) {
	// Sanity check
	if len(msg.Params) < 3 {
		log.Debug().Msgf("[upstream %s] Ignoring RPL 324 due to invalid paramaters", b.GetUpstreamConn().Config.Server)
		return false, nil
	}

	channel := msg.Params[1]
	modes := msg.Params[2:]

	log.Debug().Msgf("[upstream %s] Setting modes for channel %s to %v", b.GetUpstreamConn().Config.Server, channel, modes)

	b.SetChannelMode(channel, modes)

	// Forward message to client
	b.BroadcastToClients(msg)

	return true, nil
}
