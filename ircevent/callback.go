package ircevent

import "github.com/ergochat/irc-go/ircmsg"

func (us *UpstreamConnection) RegisterCallback(command string, cb UpstreamCommandHandler) {
	// This could probably be more performant with more fine tuned locking,
	// however, callbacks are not registered that often so.
	us.callback_mu.Lock()
	defer us.callback_mu.Unlock()

	// Append callback
	us.callbacks[command] = append(us.callbacks[command], cb)

	us.logger.Trace().Msgf("Registering callback %d for '%s'", len(us.callbacks[command])-1, command)
}

func (us *UpstreamConnection) handleCallback(msg ircmsg.Message) {
	us.logger.Trace().Msgf("Handling callback for '%s'", msg.Command)

	// Grab callback lock
	us.callback_mu.RLock()
	defer us.callback_mu.RUnlock()

	// Grab callback
	callbacks := us.callbacks[msg.Command]
	if callbacks == nil {
		us.logger.Warn().Msgf("'%s' has no callbacks registered!", msg.Command)
		return
	}

	// Call callback
	for index, cb := range callbacks {
		us.logger.Trace().Msgf("Calling callback %d for command '%s'", index, msg.Command)
		keepGoing, err := cb(msg)
		if err != nil {
			us.logger.Error().Msgf("error occurred in callback %d: %+v", index, err)
			continue
		}

		if !keepGoing {
			us.logger.Debug().Msgf("Skipping additional callbacks to to keepGoing = false")
			return
		}
	}
}
