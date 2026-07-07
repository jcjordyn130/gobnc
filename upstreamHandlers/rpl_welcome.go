package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

// wtf?
func Handle001(b Router, msg ircmsg.Message) error {
	log.Debug().Msgf("[upstream %s] Received 001 (RPL_WELCOME) from upstream server", b.GetUpstreamConn().Server)

	// Start channel autojoin
	go func() {
		err := b.JoinAutoJoinChannels()
		if err != nil {
			log.Debug().Msgf("[upstream %s] Error joining autojoin channels: %v", b.GetUpstreamConn().Server, err)
		}
	}()

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
