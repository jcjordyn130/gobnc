package downstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandleJOIN(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// XXX: this *might* be safe, idk if the library checks for valid commands for us
	channelName := msg.Params[0]

	if b.IsJoined(channelName) {
		// We're already here, give user the cached data
		b.SendDownstreamJoin(ds, channelName)
	} else {
		// We're not here, get upstream to join
		b.GetUpstreamConn().Join(channelName)
	}

	return nil
}
