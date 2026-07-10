// core/bouncer.go
// This file implements the Bouncer structure which contains the core methods/variables for
// downstream connections.
package core

import (
	"bouncer/database"
	"fmt"
	"strings"
	"time"

	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

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
	if b.upstreamConn == nil {
		log.Debug().Msg("Returning NULL upstreamConn!")
	}

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
	log.Debug().Msgf("[downstream %s] Routing to command handler for: %s", ds.Conn.RemoteAddr(), msg.Command)
	if handler, exists := b.routes[msg.Command]; exists {
		// Pass the bouncer instance into the handler
		// TODO: pass errors?
		go handler(b, ds, msg)
		return nil
	} else {
		return &HandlerNotFound{msg.Command}
	}
}

func (b *Bouncer) SendHistory(channel *string, ds *DownstreamConnection) {
	var msgChan <-chan database.ChatMessage
	var errChan <-chan error

	if channel != nil {
		log.Debug().Msgf("[downstream %s] Sending backlog for %s", ds.Conn.RemoteAddr(), *channel)
		multiFilter := map[string]string{
			"target": *channel,
		}

		msgChan, errChan = b.DB.AsyncSearchMessages(ds.Ctx, multiFilter, 999999)
	} else {
		log.Debug().Msgf("[downstream %s] Sending backlog with no filter!!!", ds.Conn.RemoteAddr())
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
		err := ds.SendToClient(formsg)
		if err != nil {
			log.Debug().Msgf("[downstream %s] Write failed during history, disconnecting", ds.Conn.RemoteAddr())
			b.DisconnectDownstreamConnection(ds, "broken client")
			return // Exit the loop
		}
	}

	if err := <-errChan; err != nil {
		log.Debug().Msgf("[downstream %s] History stream ended: %v", ds.Conn.RemoteAddr(), err)
	}
}

func (b *Bouncer) BroadcastToClients(msg ircmsg.Message) {
	if len(b.DownstreamConnections) == 0 {
		log.Debug().Msgf("[bouncer] No downstream clients to broadcast to!")
		return
	}

	log.Debug().Msgf("[bouncer] Broadcasting message to %d clients", len(b.DownstreamConnections))

	// Copy connections as net.Conn.Write can block
	log.Debug().Msgf("[bouncer] Locking DownstreamConnections and grabbing clients!")
	b.ds_mu.RLock()
	activeClients := make([]*DownstreamConnection, len(b.DownstreamConnections))
	copy(activeClients, b.DownstreamConnections)
	b.ds_mu.RUnlock()

	for _, ds := range activeClients {
		if ds.HandshakeComplete {
			log.Debug().Msgf("[downstream %s] Forwarding message to active client!", ds.Conn.RemoteAddr())
		} else {
			log.Debug().Msgf("[downstream %s] Skipping message to client that has not completed handshake!", ds.Conn.RemoteAddr())
			continue
		}

		// Copy message so we can modify it for each client if needed (e.g., add server-time)
		clientMsg := b.spoofSource(ds, msg)

		go ds.SendToClient(clientMsg)
	}
}

func (b *Bouncer) spoofSource(ds *DownstreamConnection, msg ircmsg.Message) ircmsg.Message {
	// Strict clients drop numerics if the prefix doesn't match what the bouncer is, so lets spoof it
	if len(msg.Command) == 3 && msg.Command[0] >= '0' && msg.Command[0] <= '9' {
		log.Debug().Msgf("[downstream %s] Spoofing prefix for numeric %s to bouncer server name %s", ds.Conn.RemoteAddr(), msg.Command, b.ServerName)
		msg.Source = b.ServerName

		if len(msg.Params) > 0 {
			log.Debug().Msgf("[downstream %s] Replacing first param %s with downstream nick %s", ds.Conn.RemoteAddr(), msg.Params[0], ds.Nick)
			newParams := make([]string, len(msg.Params))
			copy(newParams, msg.Params)
			newParams[0] = ds.Nick // Replace the first param with the client's nick
			msg.Params = newParams
		}
	}

	return msg
}

func (b *Bouncer) LogToDB(msg ircmsg.Message) {
	// Write message to database for backlog
	b.DB.LogQueue <- msg
}

func (b *Bouncer) ChangeDownstreamNick(newnick string) {
	for _, ds := range b.DownstreamConnections {
		log.Debug().Msgf("Changing nick from %s to %s", ds.Nick, newnick)

		// Create NICK command
		nickmsg := ircmsg.MakeMessage(nil, ds.Nick, "NICK", newnick)
		ds.SendToClient(nickmsg)

		// Change internal state
		ds.Nick = newnick
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

func (b *Bouncer) DeleteChannel(channel string) {
	b.mu.Lock()
	_, exists := b.Channels[channel]
	defer b.mu.Unlock()

	// Hoeh?
	if !exists {
		log.Debug().Msgf("DeleteChannel ran with non-existent channel!")
		return
	}

	delete(b.Channels, channel)
}

func (b *Bouncer) SendDownstreamJoin(ds *DownstreamConnection, channel string) {
	b.mu.RLock()
	chState, exists := b.Channels[channel]
	b.mu.RUnlock()

	if !exists {
		log.Trace().Msgf("[downstream %s] Downstream JOIN requested for channel we are not in", ds.Conn.RemoteAddr())
		return // We aren't actually in this channel
	}

	prefix := fmt.Sprintf("%s!user@bouncer", ds.Nick)

	// 1. The JOIN Confirmation
	joinMsg := ircmsg.MakeMessage(nil, prefix, "JOIN", channel)
	ds.SendToClient(joinMsg)

	// 2. The Topic (332)
	if chState.Topic != "" {
		topicMsg := ircmsg.MakeMessage(nil, b.ServerName, "332", ds.Nick, channel, chState.Topic)
		ds.SendToClient(topicMsg)
	}

	// Send /NAMES
	b.sendNamesList(ds, chState)

	// We hath joined!!! send history
	b.SendHistory(&channel, ds)
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

func (b *Bouncer) DisconnectDownstreamConnection(ds *DownstreamConnection, reason string) {
	// Make sure we were given a valid pointer
	if ds == nil {
		panic("ds = nil")
	}

	// Set default reason
	if reason == "" {
		reason = "No Reason Given"
	}

	// Format the mandatory ERROR message trailing string
	errorPayload := fmt.Sprintf("Closing Link: %s (Quit: %s)", ds.Nick, reason)

	// Construct the message using ircmsg.MakeMessage
	// ERROR takes exactly one parameter containing the explanation.
	msg := ircmsg.MakeMessage(nil, "", "ERROR", errorPayload)

	// Intentionally ignoring error here
	_ = ds.SendToClient(msg)
	log.Debug().Msgf("[downstream %s] Sending ERROR to breaking client", ds.Conn.RemoteAddr())

	// Close connection
	ds.Close()

	// Cancel context
	if ds.Cancel != nil {
		ds.Cancel()
	} else {
		panic("Downstream already cancelled???")
	}

	log.Debug().Msgf("[downstream %s] Removing DownstreamConnection", ds.Conn.RemoteAddr())

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
		log.Debug().Msgf("[downstream %s] RemoveDownstreamConnection called with invalid connection", ds.Conn.RemoteAddr())
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
			prefix := modeToPrefix(char)
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

			log.Debug().Msgf("[State] Channel %s | User %s | New Mode: %s", channel, targetNick, curPrefix)

		default:
			// Other channel modes like +k (password), +l (limit), +t (topic lock).
			// You can implement state tracking for these later if you want to replay
			// them on client connect, but passing them through live is usually enough.
		}
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
		log.Debug().Msgf("[downstream %s] Including ourselves in self-echo due to echo-message cap", sender.Conn.RemoteAddr())
		activeClients = append(activeClients, sender)
	}

	// 4. Send the echo to all other devices
	for _, ds := range activeClients {
		log.Debug().Msgf("[downstream %s] Echoing message to %s", sender.Conn.RemoteAddr(), ds.Conn.RemoteAddr())

		// We have to rebuild the message as normal outgoing PRIVMSGs do not have a hostmask
		echoMsg := ircmsg.MakeMessage(nil, prefix, msg.Command, msg.Params...)
		if ds.Caps["server-time"] {
			echoMsg.SetTag("time", currentTime)
		}

		ds.SendToClient(echoMsg)
	}
}

func (b *Bouncer) AddDownstreamConnection(ds *DownstreamConnection) {
	log.Debug().Msgf("[downstream %s] Adding DownstreamConnection to list!", ds.Conn.RemoteAddr())
	b.mu.Lock()
	defer b.mu.Unlock()

	b.DownstreamConnections = append(b.DownstreamConnections, ds)
}

func (b *Bouncer) JoinAutoJoinChannels() error {
	autojoinChannels, err := b.DB.GetAutoJoinChans()
	if err != nil {
		return fmt.Errorf("error retrieving autojoin channels: %w", err)
	}

	for _, channel := range autojoinChannels {
		log.Debug().Msgf("[upstream %s] Joining autojoin channel: %s", b.GetUpstreamConn().Server, channel)
		err := b.GetUpstreamConn().Join(channel)
		if err != nil {
			log.Debug().Msgf("[upstream %s] Error joining autojoin channel %s: %v", b.GetUpstreamConn().Server, channel, err)
			return err
		}

		// Wait a bit to avoid flooding the upstream server with JOINs
		// TODO: Make this configurable in the future
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// Function does not return an error as we are already shutting down.
// Therefore we just log the error and continue with the shutdown process.
func (b *Bouncer) Shutdown() {
	log.Info().Msg("Initiating graceful shutdown...")

	// 1. Disconnect all downstream clients cleanly
	// GetDownstreamConns() is safe as it returns a copy of the slice,
	// preventing deadlock when DisconnectDownstreamConnection locks the array to remove them.
	clients := b.GetDownstreamConns()
	for _, ds := range clients {
		log.Info().Msgf("Sending clean disconnect to downstream client %s", ds.Conn.RemoteAddr())
		b.DisconnectDownstreamConnection(ds, "Bouncer shutting down")
	}

	// 2. Disconnect from upstream
	if b.upstreamConn != nil && b.upstreamConn.Connected() {
		log.Info().Msg("Sending QUIT to upstream server...")
		b.upstreamConn.QuitMessage = "Shutdown() called"
		b.upstreamConn.Quit()

		// Give the TCP buffer a fraction of a second to flush the QUIT message
		// before the process exits and destroys the socket.
		time.Sleep(200 * time.Millisecond)
	}

	log.Info().Msg("Shutdown complete.")
}

func (b *Bouncer) RemoveUserFromAllChannels(nick string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.Channels {
		if _, exists := ch.Users[nick]; exists {
			// delete() is safe even if nick doesn't exist in the Users map
			delete(ch.Users, nick)
			log.Debug().Msgf("[bouncer] Removed user %s from channel %s", nick, ch.Name)
		}
	}
}
