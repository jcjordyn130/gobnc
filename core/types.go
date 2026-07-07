package core

import (
	"bouncer/database"
	"context"
	"net"
	"sync"

	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
)

// Signature for downstream command handlers
type DownstreamCommandHandler func(b *Bouncer, ds *DownstreamConnection, msg ircmsg.Message) error

// Core bouncer struct
// This holds the command handler mapping and the connection to the upstream server
type Bouncer struct {
	upstreamConn          *ircevent.Connection
	DownstreamConnections []*DownstreamConnection
	DB                    *database.DB
	routes                map[string]DownstreamCommandHandler

	// Holds a mapping of channel names to state structures
	Channels map[string]*ChannelState

	// Protects critical data structures
	mu      sync.RWMutex // Protects the Channels map
	ds_mu   sync.RWMutex // Protects the DownstreamConnections
	motd_mu sync.RWMutex // Protects the motdCache

	// Fake server name to use when broadcasting to downstream clients
	ServerName string

	// Cached MOTD
	motdCache []ircmsg.Message
}

// Downstream connection struct
// This holds the state of connected clients
type DownstreamConnection struct {
	Conn net.Conn
	Nick string

	// These hold the signal for goroutines that are using this
	// to exit on disconnect
	Ctx    context.Context
	Cancel context.CancelFunc

	// Map of server supported caps to client support
	Caps map[string]bool

	// Whether or not the handshake has completed (USER/NICK/CAP negotiation)
	// This is toggled right after RPL_001 is sent to the client
	HandshakeComplete bool
}

type ChannelState struct {
	Name  string
	Topic string
	// Key: Nickname (e.g. "Alice"), Value: Prefix (e.g. "@", "+", or "")
	Users map[string]string
	Modes string

	// This is *technically* a UNIX timestamp but it's getting formatted
	// into a string anyways, so why bother?
	CreationTime string
}
