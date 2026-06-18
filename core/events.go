package core

import "log"

func (b *Bouncer) OnUpstreamJoin(channelName string) {
	log.Printf("[bouncer] Running OpUpstreamJoin handler for %s", channelName)
	for _, client := range b.GetDownstreamConns() {
		go b.SendHistory(&channelName, client)
	}
}
