// downstreamBot/status.go
// This implements a ZNC-like *status bot through PRIVMSG commands
package downstreambot

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandlePRIVMSG(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	switch msg.Params[1] {
	case "isConnected":
		err := handleIsConnected(b, ds, msg)
		if err != nil {
			log.Printf("[downstream %s] Error sending *status response: %v", ds.Conn.RemoteAddr(), err)
		}

	case "connect":
		err := handleConnect(b, ds, msg)
		if err != nil {
			log.Printf("[downstream %s] Error sending *status response: %v", ds.Conn.RemoteAddr(), err)
		}

	case "disconnect":
		err := handleDisconnect(b, ds, msg)
		if err != nil {
			log.Printf("[downstream %s] Error sending *status response: %v", ds.Conn.RemoteAddr(), err)
		}
	}

	return nil
}
