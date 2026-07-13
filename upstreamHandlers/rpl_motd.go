package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

// Start of MOTD
func Handle375(b Router, msg ircmsg.Message) (bool, error) {
	b.ClearMOTD()
	b.CacheMOTD(msg)
	return true, nil
}

// MOTD line
func Handle372(b Router, msg ircmsg.Message) (bool, error) {
	b.CacheMOTD(msg)
	return true, nil
}

// End of MOTD
func Handle376(b Router, msg ircmsg.Message) (bool, error) {
	b.CacheMOTD(msg)
	return true, nil
}

// MOTD Missing
func Handle422(b Router, msg ircmsg.Message) (bool, error) {
	b.ClearMOTD()
	b.CacheMOTD(msg)
	return true, nil
}
