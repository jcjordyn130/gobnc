// downstreamBot/connect.go
// This implements the "connect" command
package downstreambot

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func handleConnect(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	log.Printf("[downstream %s] Client requested upstream connect!", ds.Conn.RemoteAddr())
	upstream := b.GetUpstreamConn()

	// We can't connect to an already connected server
	if upstream.Connected() {
		rawmsg := ircmsg.MakeMessage(nil, "*status", "PRIVMSG", "Upstream is already connected!")
		b.SendToClient(ds, rawmsg)
		return nil
	}

	// Connect... again
	//_ = b.ConnectToServer(b.UpstreamConn)
	upstream.Reconnect()

	// Start upstream handler loop
	go upstream.Loop()

	return nil
}
