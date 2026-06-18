// downstreamBot/connect.go
// This implements the "connect" command
package downstreambot

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func handleDisconnect(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	log.Printf("[downstream %s] Client requested upstream disconnect!", ds.Conn.RemoteAddr())
	upstream := b.GetUpstreamConn()

	// We can't connect to an already connected server
	if !upstream.Connected() {
		rawmsg := ircmsg.MakeMessage(nil, "*status", "PRIVMSG", "Upstream is already not connected!")
		b.SendToClient(ds, rawmsg)
		return nil
	}

	// Send QUIT message
	// This should cause the server to close our socket
	quitmsg := ircmsg.MakeMessage(nil, upstream.Nick, "QUIT", "brb")
	upstream.SendIRCMessage(quitmsg)

	return nil
}
