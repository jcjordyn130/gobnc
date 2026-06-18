package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

// Start of MOTD
func Handle375(b Router, msg ircmsg.Message) error {
	b.ClearMOTD()
	b.CacheMOTD(msg)
	return nil
}

// MOTD line
func Handle372(b Router, msg ircmsg.Message) error {
	b.CacheMOTD(msg)
	return nil
}

// End of MOTD
func Handle376(b Router, msg ircmsg.Message) error {
	b.CacheMOTD(msg)
	return nil
}
