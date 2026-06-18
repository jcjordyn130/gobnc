package downstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandleNIK(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// Handle weird edge cases
	// TODO: maybe the ircevent library handles this for us?
	if len(msg.Params) < 1 {
		log.Printf("[downstream %s] NICK command with no nickname receieved?", ds.Conn.RemoteAddr())
		return nil
	}

	upstream := b.GetUpstreamConn()

	if !upstream.Connected() {
		log.Printf("[downstream %s] NICK command attempted with no upstream connection!", ds.Conn.RemoteAddr())
		return nil
	} else {
		// Change our nickname with the upstream server
		log.Printf("[downstream %s] Changing nickname to %s", ds.Conn.RemoteAddr(), msg.Params[0])
		upstream.SetNick(msg.Params[0])
	}

	return nil
}
