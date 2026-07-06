package downstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandlePART(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// XXX: this *might* be safe, idk if the library checks for valid commands for us
	channelName := msg.Params[0]

	if b.IsJoined(channelName) {
		// Delete channel
		b.DeleteChannel(channelName)

		// Upstream PART
		err := b.GetUpstreamConn().SendIRCMessage(msg)
		if err != nil {
			log.Printf("[downstream %s] Error sending upstream part for %s", ds.Conn.RemoteAddr(), channelName)
		}
	} else {
		// We're not here? wot m8
		log.Printf("[downstream %s] PART receieved for channel we are not in!", ds.Conn.RemoteAddr())
	}

	return nil
}
