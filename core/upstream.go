// core/upstream.go
// This file contains the core of the Bouncer structure as it relates to the upstream connection
package core

import (
	"bouncer/upstreamHandlers"

	"github.com/rs/zerolog/log"

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
	upstreamHandlers.Register(b, "PART", upstreamHandlers.HandlePART)
	upstreamHandlers.Register(b, "001", upstreamHandlers.Handle001) // RPL_WELCOME
	upstreamHandlers.Register(b, "477", upstreamHandlers.Handle477) // Cannot join channel (unregistered)
	upstreamHandlers.Register(b, "900", upstreamHandlers.Handle900) // RPL_LOGGEDIN
	upstreamHandlers.Register(b, "302", upstreamHandlers.Handle302) // RPL_USERHOST

	upstreamHandlers.Register(b, "324", upstreamHandlers.Handle324) // Channel modes (e.g. +Cnst)
	upstreamHandlers.Register(b, "367", upstreamHandlers.Handle367) // Banlist item
	upstreamHandlers.Register(b, "368", upstreamHandlers.Handle368) // End of banlist

	// WHOIS related handlers
	upstreamHandlers.Register(b, "311", upstreamHandlers.Handle311)
	upstreamHandlers.Register(b, "319", upstreamHandlers.Handle319)
	upstreamHandlers.Register(b, "312", upstreamHandlers.Handle312)
	upstreamHandlers.Register(b, "671", upstreamHandlers.Handle671)
	upstreamHandlers.Register(b, "338", upstreamHandlers.Handle338)
	upstreamHandlers.Register(b, "317", upstreamHandlers.Handle317)
	upstreamHandlers.Register(b, "318", upstreamHandlers.Handle318)
	upstreamHandlers.Register(b, "330", upstreamHandlers.Handle330)

	// Connect to the upstream server
	err = conn.Connect()
	if err != nil {
		log.Debug().Msgf("Upstream connection failure: %v", err)
		return err
	}

	return nil
}
