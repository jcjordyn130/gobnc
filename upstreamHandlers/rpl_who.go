package upstreamHandlers

import (
	"bouncer/models"
	"strings"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

// RPL_352 -- /WHO line
func Handle352(b Router, msg ircmsg.Message) (bool, error) {
	if len(msg.Params) < 8 {
		log.Warn().Msgf("[upstream %s] Invalid /WHO line receieved", b.GetUpstreamConn().Config.Server)
		return false, nil
	}

	// Required paramaters
	username := msg.Params[2]
	host := msg.Params[3]
	server := msg.Params[4]
	nick := msg.Params[5]
	flags := msg.Params[6]

	// Extra paramaters
	trailing := msg.Params[7]
	realname := trailing
	parts := strings.SplitN(trailing, " ", 2)
	if len(parts) == 2 {
		realname = parts[1]
	}

	// Process flags
	isAway := strings.ContainsRune(flags, 'G')
	isIRCOp := strings.ContainsRune(flags, '*')

	// Use the universal helper to update ONLY the WHO properties
	b.ModifyUser(nick, func(u *models.UserState) {
		u.Username = username
		u.Host = host
		u.Server = server
		u.Realname = realname
		u.Away = isAway
		u.IRCOp = isIRCOp

		if !isAway {
			u.AwayMessage = ""
		}
	})

	b.BroadcastToClients(msg)
	return true, nil
}

// RPL_315 -- End of /WHO
func Handle315(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}
