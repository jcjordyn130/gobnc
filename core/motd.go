package core

import "github.com/ergochat/irc-go/ircmsg"

func (b *Bouncer) ClearMOTD() {
	b.motd_mu.Lock()
	defer b.motd_mu.Unlock()
	b.motdCache = nil
}

func (b *Bouncer) CacheMOTD(msg ircmsg.Message) {
	b.motd_mu.Lock()
	defer b.motd_mu.Unlock()
	b.motdCache = append(b.motdCache, msg)
}

func (b *Bouncer) SendCachedMOTD(ds *DownstreamConnection) {
	b.motd_mu.RLock()
	// We make a local copy so we can iterate without holding the lock during I/O
	msgs := make([]ircmsg.Message, len(b.motdCache))
	copy(msgs, b.motdCache)
	b.motd_mu.RUnlock()

	for _, msg := range msgs {
		msg := b.spoofSource(ds, msg)
		ds.SendToClient(msg)
	}
}
