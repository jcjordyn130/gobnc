package events

import "github.com/rs/zerolog/log"

// Start is called when the network is initialized
func (s *NetworkSession) Start() {
	// Listen for upstream events in the background
	go s.eventLoop()

	// Start upstream loop
	go s.upstream.conn.Loop()
}

func (s *NetworkSession) eventLoop() {
	for event := range s.events {
		log.Debug().Msgf("[eventLoop] got event %s with channel %s", event.Type, event.Channel)

		switch event.Type {
		case "CHANNEL_SYNCED":
			// A channel just finished joining upstream!
			// Iterate through all attached downstream clients and send them history.
			for _, ds := range s.clients {
				// We fire a goroutine for each client so we don't block the event loop
				go s.bouncer.SendHistory(&event.Channel, ds)
			}
		}
	}
}
