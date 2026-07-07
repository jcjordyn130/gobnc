package core

import "github.com/rs/zerolog/log"

func (b *Bouncer) OnUpstreamJoin(channelName string) {
	log.Debug().Msgf("[bouncer] Running OpUpstreamJoin handler for %s", channelName)
	for _, client := range b.GetDownstreamConns() {
		go b.SendHistory(&channelName, client)
	}
}
