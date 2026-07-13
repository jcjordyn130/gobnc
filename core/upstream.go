// core/upstream.go
package core

import (
	"bouncer/ircevent"
	"bouncer/upstreamHandlers"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func (b *Bouncer) ConnectToServer(conn *ircevent.UpstreamConnection) (err error) {
	// Store the upstream connection for later use
	b.upstreamConn = conn

	// Map of IRC commands to their handler functions
	// If upstreamHandlers exports a specific type (e.g., upstreamHandlers.Handler),
	// you can replace the func signature below with that type.
	handlers := map[string]func(b upstreamHandlers.Router, msg ircmsg.Message) (bool, error){
		// Basic messaging and connection
		"PING":    upstreamHandlers.HandlePING,
		"PRIVMSG": upstreamHandlers.HandlePRIVMSG,
		"NOTICE":  upstreamHandlers.HandleNOTICE,
		"NICK":    upstreamHandlers.HandleNICK,
		"QUIT":    upstreamHandlers.HandleQUIT,

		// Channel operations
		"JOIN":  upstreamHandlers.HandleJOIN,
		"PART":  upstreamHandlers.HandlePART,
		"TOPIC": upstreamHandlers.HandleTOPIC,
		"KICK":  upstreamHandlers.HandleKICK,
		"MODE":  upstreamHandlers.HandleMODE,

		// Numerics: Connection & Welcome
		"001": upstreamHandlers.Handle001, // RPL_WELCOME
		"900": upstreamHandlers.Handle900, // RPL_LOGGEDIN

		// Numerics: Channel state & lists
		"324": upstreamHandlers.Handle324, // Channel modes
		"329": upstreamHandlers.Handle329, // Channel creation timestamp
		"332": upstreamHandlers.Handle332, // Topic on join
		"353": upstreamHandlers.Handle353, // NAMES
		"366": upstreamHandlers.Handle366, // End of NAMES
		"367": upstreamHandlers.Handle367, // Banlist item
		"368": upstreamHandlers.Handle368, // End of banlist

		// Numerics: Errors & Restrictions
		"421": upstreamHandlers.Handle421, // RPL_UNKNOWNCOMMAND
		"473": upstreamHandlers.Handle473, // Cannot join channel (+i)
		"474": upstreamHandlers.Handle474, // Cannot join channel (+b)
		"477": upstreamHandlers.Handle477, // Cannot join channel (unregistered)

		// Numerics: WHOIS
		"311": upstreamHandlers.Handle311,
		"312": upstreamHandlers.Handle312,
		"317": upstreamHandlers.Handle317,
		"318": upstreamHandlers.Handle318,
		"319": upstreamHandlers.Handle319,
		"330": upstreamHandlers.Handle330,
		"338": upstreamHandlers.Handle338,
		"402": upstreamHandlers.Handle402,
		"671": upstreamHandlers.Handle671,

		// Numerics: WHO
		"352": upstreamHandlers.Handle352, // WHO line
		"315": upstreamHandlers.Handle315, // end of who

		// Numerics: MOTD
		"372": upstreamHandlers.Handle372, // MOTD line
		"375": upstreamHandlers.Handle375, // MOTD Start
		"376": upstreamHandlers.Handle376, // MOTD End
		"422": upstreamHandlers.Handle422, // MOTD missing

		// Numerics: Away & Identification
		"302": upstreamHandlers.Handle302, // RPL_USERHOST
		"305": upstreamHandlers.Handle305, // RPL_305 No Longer Away
		"306": upstreamHandlers.Handle306, // RPL_306 Client Away
		"396": upstreamHandlers.Handle396, // Vhost / Nickserv
	}

	// Register all upstream handlers dynamically
	for cmd, handler := range handlers {
		log.Trace().Msgf("[bouncer] Registering upstream handler for %s", cmd)
		upstreamHandlers.Register(b, cmd, handler)
	}

	// Connect to the upstream server
	err = conn.Connect()
	if err != nil {
		log.Debug().Msgf("Upstream connection failure: %v", err)
		return err
	}

	return nil
}
