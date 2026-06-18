package downstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandleQUIT(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// XXX: this *might* be safe, idk if the library checks for valid commands for us
	quitMsg := msg.Params[0]
	log.Printf("[downstream %s] QUIT received with message %s", ds.Conn.RemoteAddr(), quitMsg)

	b.RemoveDownstreamConnection(ds)
	return nil
}
