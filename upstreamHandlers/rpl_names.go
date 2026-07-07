package upstreamHandlers

import (
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle353(b Router, msg ircmsg.Message) error {
	// Sanity check
	if len(msg.Params) < 3 {
		log.Debug().Msgf("[upstream %s] Ignoring RPL 353 due to invalid paramaters", b.GetUpstreamConn().Server)
		return nil
	}

	channel := msg.Params[2]
	nameStr := msg.Params[3]

	log.Debug().Msgf("[upstream %s] Processing NAMES list for %s", b.GetUpstreamConn().Server, channel)

	// Splice the space-separated (per IRC standards) list of names
	users := strings.Split(strings.TrimSpace(nameStr), " ")
	b.AddChannelUsers(channel, users)

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}

func Handle366(b Router, msg ircmsg.Message) error {
	log.Debug().Msgf("[upstream %s] End of NAMES list for %s", b.GetUpstreamConn().Server, msg.Params[1])
	// Forward message to client
	b.BroadcastToClients(msg)

	// Run event handler if we have a channel
	if len(msg.Params) >= 2 {
		channelName := msg.Params[1]
		b.OnUpstreamJoin(channelName)
	}

	return nil
}
