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
	Accounts map[string]*UserAccount

	DB     *database.DB
	routes map[string]DownstreamCommandHandler

	// Protects critical data structures
	mu sync.RWMutex // Protects the Channels map

	// Fake server name to use when broadcasting to downstream clients
	ServerName string
}

type UserAccount struct {
	Username string
	Password string // HASHED

	// Networks maps a friendly name (ex. 'libera' or 'ircat') to the connection type
	Networks map[string]*Network

	// Clients are the active downstream connections
	Clients []*DownstreamConnection

	// State!!!
	mu sync.RWMutex
}

type Network struct {
	Name         string // Friendly name (ex. 'libera' or 'irccat')
	UpstreamConn *ircevent.Connection

	// Old bouncer state specific to this network to avoid collisions
	// and excessive locking.
	Channels map[string]*models.ChannelState
	Peers    map[string]*models.UserState

	// Cached MOTD
	motdCache []ircmsg.Message

	// State!!!!
	mu sync.RWMutex
}

// Downstream connection struct
// This holds the state of connected clients
type DownstreamConnection struct {
	Conn net.Conn
	Nick string

	// These hold the specific network and account this downstream is bound to
	// We do NOT support IRCv3 multiplexing
	Account       *UserAccount
	ActiveNetwork *Network

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
