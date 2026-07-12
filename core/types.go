package core

import (
	"bouncer/database"
	"bouncer/models"
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
	Channels map[string]*models.ChannelState

	// Holds a mapping of nicknames to state structures
	Users map[string]*models.UserState

	// Protects critical data structures
	mu      sync.RWMutex // Protects the Channels map
	ds_mu   sync.RWMutex // Protects the DownstreamConnections
	motd_mu sync.RWMutex // Protects the motdCache
	user_mu sync.RWMutex // Protects the Users map

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
	// This is toggled after CAP END if clients support caps
	// or USER if the client does NOT support caps
	HandshakeComplete bool

	// This is set when we get CAP {LS, REQ} and unset when we get CAP END
	// If the client does NOT support caps then it is NICK and USER
	//
	// This is purly used to control the handshake loop.
	HandshakeInProgress bool

	// CAP version supported
	CapVersionSupported string

	// internal channel to use for messages
	//msgChan chan ircmsg.Message
	msgChan chan []byte
}
