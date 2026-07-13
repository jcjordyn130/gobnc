package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle329(b Router, msg ircmsg.Message) (bool, error) {
	// Sanity check
	if len(msg.Params) < 2 {
		log.Debug().Msgf("[upstream %s] Ignoring RPL 329 due to invalid paramaters", b.GetUpstreamConn().Config.Server)
		return false, nil
	}

	channel := msg.Params[1]
	createTime := msg.Params[2]

	log.Debug().Msgf("[upstream %s] Channel %s created at %s", b.GetUpstreamConn().Config.Server, channel, createTime)

	b.SetChannelCreationTime(channel, createTime)

	// Forward message to client
	b.BroadcastToClients(msg)

	return true, nil
}
