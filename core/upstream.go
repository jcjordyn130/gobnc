// core/upstream.go
// This file contains the core of the Bouncer structure as it relates to the upstream connection
package core

import (
	"bouncer/upstreamHandlers"
	"log"

	"github.com/ergochat/irc-go/ircevent"
)

func (b *Bouncer) ConnectToServer(conn *ircevent.Connection) (err error) {
	// Store the upstream connection for later use
	// Update: this is the cleanest way to do it
	b.upstreamConn = conn

	// Register upstream handlers
	// These are commands send by the connected IRC servers
	upstreamHandlers.Register(b, "PING", upstreamHandlers.HandlePING)
	upstreamHandlers.Register(b, "PRIVMSG", upstreamHandlers.HandlePRIVMSG)
	upstreamHandlers.Register(b, "NOTICE", upstreamHandlers.HandleNOTICE)
	upstreamHandlers.Register(b, "NICK", upstreamHandlers.HandleNICK)
	upstreamHandlers.Register(b, "332", upstreamHandlers.Handle332) // /TOPIC on join
	upstreamHandlers.Register(b, "353", upstreamHandlers.Handle353) // /NAMES
	upstreamHandlers.Register(b, "366", upstreamHandlers.Handle366) // End of /NAMES
	upstreamHandlers.Register(b, "JOIN", upstreamHandlers.HandleJOIN)
	upstreamHandlers.Register(b, "TOPIC", upstreamHandlers.HandleTOPIC)
	upstreamHandlers.Register(b, "KICK", upstreamHandlers.HandleKICK)
	upstreamHandlers.Register(b, "MODE", upstreamHandlers.HandleMODE)
	upstreamHandlers.Register(b, "474", upstreamHandlers.Handle474) // Cannot join channel (+b)
	upstreamHandlers.Register(b, "329", upstreamHandlers.Handle329) // Channel creation timestamp
	upstreamHandlers.Register(b, "473", upstreamHandlers.Handle473) // Cannot join channel (+i)
	upstreamHandlers.Register(b, "375", upstreamHandlers.Handle375) // MOTD Start
	upstreamHandlers.Register(b, "372", upstreamHandlers.Handle372) // MOTD line
	upstreamHandlers.Register(b, "376", upstreamHandlers.Handle376) // MOTD End

	// Connect to the upstream server
	err = conn.Connect()
	if err != nil {
		log.Println("Upstream connection failure:", err)
		return err
	}

	return nil
}
