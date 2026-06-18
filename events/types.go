package events

import (
	"bouncer/core"
	"bouncer/database"

	"github.com/ergochat/irc-go/ircevent"
)

// Event represents something that happened upstream
type UpstreamEvent struct {
	Type    string // e.g., "CHANNEL_SYNCED"
	Channel string // e.g., "#golang"
}

type Upstream struct {
	conn ircevent.Connection
	// Write-only channel for emitting events
	eventsChan chan<- UpstreamEvent
}

type NetworkSession struct {
	upstream *Upstream
	bouncer  *core.Bouncer
	clients  map[string]*core.DownstreamConnection // Map of connected downstream clients
	events   chan UpstreamEvent                    // The central event bus
	db       *database.DB
}
