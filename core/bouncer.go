// core/bouncer.go
// This file implements the Bouncer structure which contains the core methods/variables for
// downstream connections.
package core

import (
	"bouncer/database"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

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
	DB                    database.DB
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

// List of IRCv3 capabilities that we support
var supportedCaps = map[string]bool{
	"server-time":  true,
	"echo-message": true,
}

func NewBouncer(upstream *ircevent.Connection) *Bouncer {
	return &Bouncer{
		upstreamConn: upstream,
		routes:       make(map[string]DownstreamCommandHandler),
		Channels:     make(map[string]*ChannelState),
		ServerName:   "bnc.jordynsblog.org",
	}
}

func (b *Bouncer) GetUpstreamConn() *ircevent.Connection {
	return b.upstreamConn
}

func (b *Bouncer) GetDownstreamConns() []*DownstreamConnection {
	b.ds_mu.RLock()
	activeClients := make([]*DownstreamConnection, 0, len(b.DownstreamConnections))
	for _, ds := range b.DownstreamConnections {
		activeClients = append(activeClients, ds)
	}
	b.ds_mu.RUnlock()

	return activeClients
}

func (b *Bouncer) SetChannelCreationTime(channel string, createTime string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, exists := b.Channels[channel]; exists {
		ch.CreationTime = createTime
	}
}

func (b *Bouncer) SetChannelMode(channel string, modeStr []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, exists := b.Channels[channel]; exists {
		ch.Modes = strings.Join(modeStr, " ")
	}
}

// Register a command handler for an IRC command from downstream
func (b *Bouncer) Register(command string, handler DownstreamCommandHandler) {
	b.routes[command] = handler
}

// Handle callbacks
func (b *Bouncer) Route(ds *DownstreamConnection, msg ircmsg.Message) error {
	log.Printf("[downstream %s] Routing to command handler for: %s", ds.Conn.RemoteAddr(), msg.Command)
	if handler, exists := b.routes[msg.Command]; exists {
		// Pass the bouncer instance into the handler
		return handler(b, ds, msg)
	} else {
		return &HandlerNotFound{msg.Command}
	}
}

func (b *Bouncer) ListenDownstream(bindAddress string) {
	log.Printf("Starting downstream accept loop for bindAddress %s", bindAddress)
	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		log.Fatalf("Failed to bind downstream port: %v", err)
	}

	log.Printf("Downstream listening on %s", bindAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Downstream accept error:", err)
			continue
		}

		log.Printf("[downstream %s] Client connected", conn.RemoteAddr())
		log.Printf("Client attached to bouncer %s", conn.RemoteAddr())
		// Add context state
		ctx, cancel := context.WithCancel(context.Background())

		downstreamConn := &DownstreamConnection{
			Conn:   conn,
			Ctx:    ctx,
			Cancel: cancel,
			Caps:   make(map[string]bool),
		}

		b.DownstreamConnections = append(b.DownstreamConnections, downstreamConn)

		// Spawn a dedicated goroutine for each client connection
		go b.handleClient(downstreamConn)
	}
}

func (b *Bouncer) SendToClient(ds *DownstreamConnection, ircmsg ircmsg.Message) error {
	ircmsgb, err := ircmsg.LineBytes()
	if err != nil {
		log.Printf("[downstream %s] Error sending response: %v", ds.Conn.RemoteAddr(), err)
		return err
	}

	_, err = ds.Conn.Write(ircmsgb)
	if err != nil {
		log.Printf("[downstream %s] Error sending response: %v", ds.Conn.RemoteAddr(), err)
		return err
	}

	return nil
}

func (b *Bouncer) SendHistory(channel *string, ds *DownstreamConnection) {
	var msgChan <-chan database.ChatMessage
	var errChan <-chan error

	if channel != nil {
		log.Printf("[downstream %s] Sending backlog for %s", ds.Conn.RemoteAddr(), *channel)
		multiFilter := map[string]string{
			"target": *channel,
		}

		msgChan, errChan = b.DB.AsyncSearchMessages(ds.Ctx, multiFilter, 999999)
	} else {
		log.Printf("[downstream %s] Sending backlog with no filter!!!", ds.Conn.RemoteAddr())
		msgChan, errChan = b.DB.AsyncGetMessages(ds.Ctx, 99999999)
	}

	for chatmsg := range msgChan {
		formsg := ircmsg.MakeMessage(nil, chatmsg.Source, "PRIVMSG", chatmsg.Target, chatmsg.Content)

		// Add server-time
		if ds.Caps["server-time"] {
			// Format UNIX epoch as ISO time as IRCv3 dictates
			timeStr := time.Unix(chatmsg.Timestamp, 0).Format(time.RFC3339Nano)
			formsg.SetTag("time", timeStr)
		}

		// If Write fails (e.g., broken pipe), initiate the cleanup
		// which will fire ds.Cancel() and kill the DB query.
		err := b.SendToClient(ds, formsg)
		if err != nil {
			log.Printf("[downstream %s] Write failed during history, disconnecting", ds.Conn.RemoteAddr())
			b.RemoveDownstreamConnection(ds)
			return // Exit the loop
		}
	}

	if err := <-errChan; err != nil {
		log.Printf("[downstream %s] History stream ended: %v", ds.Conn.RemoteAddr(), err)
	}
}

func (b *Bouncer) BroadcastToClients(msg ircmsg.Message) {
	// Copy connections as net.Conn.Write can block
	log.Printf("[bouncer] Locking DownstreamConnections and grabbing clients!")
	b.ds_mu.RLock()
	activeClients := make([]*DownstreamConnection, len(b.DownstreamConnections))
	copy(activeClients, b.DownstreamConnections)
	b.ds_mu.RUnlock()

	for _, ds := range activeClients {
		log.Printf("[downstream %s] Forwarding message to active client!", ds.Conn.RemoteAddr())
		b.SendToClient(ds, msg)
	}
}

func (b *Bouncer) LogToDB(msg ircmsg.Message) {
	// Write message to database for backlog
	b.DB.LogQueue <- msg
}

func (b *Bouncer) ChangeDownstreamNick(newnick string) {
	for _, ds := range b.DownstreamConnections {
		log.Printf("Changing nick from %s to %s", ds.Nick, newnick)

		// Create NICK command
		nickmsg := ircmsg.MakeMessage(nil, ds.Nick, "NICK", newnick)
		b.SendToClient(ds, nickmsg)

		// Change internal state
		ds.Nick = newnick
	}
}

// Helper to separate prefixes from nicknames
func parsePrefix(rawNick string) (nick string, prefix string) {
	if len(rawNick) == 0 {
		return "", ""
	}
	// Common IRC prefixes
	switch rawNick[0] {
	case '~', '&', '@', '%', '+':
		return rawNick[1:], string(rawNick[0])
	default:
		return rawNick, ""
	}
}

func (b *Bouncer) AddChannelUsers(channel string, users []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Ensure the channel exists in our map
	// TODO: cleaner syntax
	if _, exists := b.Channels[channel]; !exists {
		b.Channels[channel] = &ChannelState{
			Name:  channel,
			Users: make(map[string]string),
		}
	}

	ch := b.Channels[channel]
	for _, rawUser := range users {
		if rawUser == "" {
			continue
		}

		nick, prefix := parsePrefix(rawUser)
		ch.Users[nick] = prefix
	}
}

func (b *Bouncer) SetChannelTopic(channel, topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, exists := b.Channels[channel]; exists {
		ch.Topic = topic
	}
}

func (b *Bouncer) IsJoined(channel string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.Channels[channel]; exists {
		return true
	} else {
		return false
	}
}

func (b *Bouncer) SendDownstreamJoin(ds *DownstreamConnection, channel string) {
	b.mu.RLock()
	chState, exists := b.Channels[channel]
	b.mu.RUnlock()

	if !exists {
		return // We aren't actually in this channel
	}

	prefix := fmt.Sprintf("%s!user@bouncer", ds.Nick)

	// 1. The JOIN Confirmation
	joinMsg := ircmsg.MakeMessage(nil, prefix, "JOIN", channel)
	b.SendToClient(ds, joinMsg)

	// 2. The Topic (332)
	if chState.Topic != "" {
		topicMsg := ircmsg.MakeMessage(nil, b.ServerName, "332", ds.Nick, channel, chState.Topic)
		b.SendToClient(ds, topicMsg)
	}

	// 3. The Names List (353)
	// We rebuild the space-separated string: "@Alice +Bob Charlie"
	var nameParts []string
	b.mu.RLock() // Lock again just to read the map safely
	for nick, userPrefix := range chState.Users {
		nameParts = append(nameParts, userPrefix+nick)
	}
	b.mu.RUnlock()

	// Note: For massive channels, you'd want to split nameParts into chunks of ~15
	// so the IRC line doesn't exceed 512 bytes. For now, strings.Join is fine.
	namesStr := strings.Join(nameParts, " ")

	namesMsg := ircmsg.MakeMessage(nil, b.ServerName, "353", ds.Nick, "=", channel, namesStr)
	b.SendToClient(ds, namesMsg)

	// 4. End of Names (366)
	endMsg := ircmsg.MakeMessage(nil, b.ServerName, "366", ds.Nick, channel, "End of /NAMES list.")
	b.SendToClient(ds, endMsg)

	// We hath joined!!! send history
	b.SendHistory(&channel, ds)
}

func (b *Bouncer) DeleteChannel(channel string) {
	delete(b.Channels, channel)
}

func (b *Bouncer) RemoveUserFromChannel(channel string, user string) {
	// Get channel state
	channelS := b.Channels[channel]
	if channelS == nil {
		return
	}

	// Remove user
	delete(channelS.Users, user)
}

func (b *Bouncer) RemoveDownstreamConnection(ds *DownstreamConnection) {
	// Make sure we were given a valid pointer
	if ds == nil {
		panic("ds = nil")
	}

	// Cancel context
	if ds.Cancel != nil {
		ds.Cancel()
	} else {
		log.Printf("Downstream already cancelled???")
	}

	// Close connection
	if ds.Conn != nil {
		_ = ds.Conn.Close()
	}

	log.Printf("[downstream %s] Removing DownstreamConnection", ds.Conn.RemoteAddr())

	// Start touching the array
	b.ds_mu.Lock()
	defer b.ds_mu.Unlock()

	// Find the index of the DownstreamConnection
	targetIndex := -1
	for i, search_ds := range b.DownstreamConnections {
		if search_ds == ds {
			targetIndex = i
			break
		}
	}

	// It wasn't found???
	if targetIndex == -1 {
		log.Printf("[downstream %s] RemoveDownstreamConnection called with invalid connection", ds.Conn.RemoteAddr())
		return
	}

	// Swap and pop
	b.DownstreamConnections[targetIndex] = b.DownstreamConnections[len(b.DownstreamConnections)-1]
	b.DownstreamConnections[len(b.DownstreamConnections)-1] = nil // This is needed to avoid a leak
	b.DownstreamConnections = b.DownstreamConnections[:len(b.DownstreamConnections)-1]
}

// ApplyModes updates the internal user prefix map based on incoming MODE changes
func (b *Bouncer) ApplyModes(channel string, modeStr string, args []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check to make sure the channel *exists* first
	ch, exists := b.Channels[channel]
	if !exists {
		return
	}

	adding := true
	argIndex := 0

	for _, char := range modeStr {
		switch char {
		// Process add/remove
		case '+':
			adding = true
		case '-':
			adding = false

		// Process actual mode char
		case 'o', 'v', 'h', 'a', 'q':
			// These modes take an argument (the target user's nick)
			if argIndex >= len(args) {
				continue // Malformed command from server
			}

			targetNick := args[argIndex]
			argIndex++

			// Update the user's prefix in our state
			prefix := b.modeToPrefix(char)
			curPrefix := ch.Users[targetNick]

			if adding {
				// Note: A real IRC server can have multiple prefixes (e.g., +@).
				// For a basic bouncer, just storing the highest privilege is usually enough.
				curPrefix = curPrefix + prefix
			} else {
				// If removing a privilege, we revert them to a standard user
				curPrefix = strings.ReplaceAll(curPrefix, prefix, "")
			}

			ch.Users[targetNick] = curPrefix

			log.Printf("[State] Channel %s | User %s | New Mode: %s", channel, targetNick, curPrefix)

		default:
			// Other channel modes like +k (password), +l (limit), +t (topic lock).
			// You can implement state tracking for these later if you want to replay
			// them on client connect, but passing them through live is usually enough.
		}
	}
}

// Helper to map IRC mode characters to NAMES list prefixes
func (b *Bouncer) modeToPrefix(mode rune) string {
	switch mode {
	case 'o':
		return "@" // Operator
	case 'v':
		return "+" // Voice
	case 'h':
		return "%" // Half-Op
	case 'a':
		return "&" // Admin
	case 'q':
		return "~" // Founder
	default:
		return ""
	}
}

// EchoToOtherClients routes a user's outgoing message to all of their other connected devices.
func (b *Bouncer) EchoToOtherClients(sender *DownstreamConnection, msg ircmsg.Message) {
	// WARN: IRCv3 has echo-message cap that does this, ensure we don't double send

	// Format the source so it's like a normal prefix
	prefix := fmt.Sprintf("%s!user@%s", sender.Nick, b.ServerName)

	// Grab current time to add to bounced messages if we support server-time
	currentTime := time.Now().UTC().Format(time.RFC3339Nano)

	// 3. Extract the active connections quickly to avoid locking during network I/O
	b.ds_mu.RLock()
	activeClients := make([]*DownstreamConnection, 0, len(b.DownstreamConnections))
	for _, ds := range b.DownstreamConnections {
		// Skip the client that originally sent the message!
		// Otherwise that singular client would get double messages lmao
		if ds != sender {
			activeClients = append(activeClients, ds)
		}
	}
	b.ds_mu.RUnlock()

	// Add ourselves if we have echo-message support
	if sender.Caps["echo-message"] {
		log.Printf("[downstream %s] Including ourselves in self-echo due to echo-message cap", sender.Conn.RemoteAddr())
		activeClients = append(activeClients, sender)
	}

	// 4. Send the echo to all other devices
	for _, ds := range activeClients {
		log.Printf("[downstream %s] Echoing message to %s", sender.Conn.RemoteAddr(), ds.Conn.RemoteAddr())

		// We have to rebuild the message as normal outgoing PRIVMSGs do not have a hostmask
		echoMsg := ircmsg.MakeMessage(nil, prefix, msg.Command, msg.Params...)
		if ds.Caps["server-time"] {
			echoMsg.SetTag("time", currentTime)
		}

		b.SendToClient(ds, echoMsg)
	}
}
