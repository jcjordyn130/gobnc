package upstreamHandlers

import (
	"strings"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandleMODE(b Router, msg ircmsg.Message) (bool, error) {
	target := msg.Params[0]
	modeStr := msg.Params[1]
	modeArgs := msg.Params[2:]

	// Forward to other clients
	b.BroadcastToClients(msg)

	// Ignore user modes other than for broadcast
	if !strings.ContainsAny(string(target[0]), "#&!+") {
		// It's a User mode (e.g., MODE MyNick +i).
		// We usually just pass these through and don't track them tightly.
		return true, nil
	}

	// Update channel state
	b.ApplyModes(target, modeStr, modeArgs)
	return true, nil
}
