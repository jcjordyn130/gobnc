package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle332(b Router, msg ircmsg.Message) error {
	// Sanity check
	if len(msg.Params) < 2 {
		log.Debug().Msgf("[upstream %s] Ignoring RPL 332 due to invalid paramaters", b.GetUpstreamConn().Server)
		return nil
	}

	channel := msg.Params[1]
	topic := msg.Params[2]

	log.Debug().Msgf("[upstream %s] Setting topic for channel %s to: %s", b.GetUpstreamConn().Server, channel, topic)

	b.SetChannelTopic(channel, topic)

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
