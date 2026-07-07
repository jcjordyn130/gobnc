package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

// wtf?
func Handle001(b Router, msg ircmsg.Message) error {
	log.Debug().Msgf("[upstream %s] Received 001 (RPL_WELCOME) from upstream server", b.GetUpstreamConn().Server)

	// Forward message to client
	b.BroadcastToClients(msg)

	// Start channel autojoin
	// Potentially this could be moved to after the RPL_005 or RPL_376 (end of MOTD) is received, but for now this is fine.
	// Alternatives are also waiting for the server to send us our mode, or wait until the first PING.
	go func() {
		err := b.JoinAutoJoinChannels()
		if err != nil {
			log.Debug().Msgf("[upstream %s] Error joining autojoin channels: %v", b.GetUpstreamConn().Server, err)
		}
	}()

	return nil
}
