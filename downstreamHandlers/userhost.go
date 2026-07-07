package downstreamHandlers

import (
	"bouncer/core"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func DisConHandleUSERHOST(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	log.Warn().Msgf("[downstream %s] Received USERHOST while disconnected from upstream, NOT IMPLEMENTED", ds.Conn.RemoteAddr())
	return nil
}

func HandleUSERHOST(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// Ignore empty requests
	if len(msg.Params) == 0 {
		return nil
	}

	upstream := b.GetUpstreamConn()

	// Handle USERHOST
	if !upstream.Connected() {
		return DisConHandleUSERHOST(b, ds, msg)
	} else {
		// Maybe this needs an upstream handler, but for now just echo to upstream
		return b.GetUpstreamConn().SendIRCMessage(msg)
	}
}
