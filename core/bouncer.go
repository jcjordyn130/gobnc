// core/bouncer.go
// This file implements the Bouncer structure which contains the core methods/variables for
// downstream connections.
package core

import (
	"bouncer/database"
	"bouncer/models"

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
	"multi-prefix": true,
}

func NewBouncer(upstream *ircevent.Connection) *Bouncer {
	return &Bouncer{
		UpstreamConnections: make([]*UpstreamConnection, 0),
		routes:              make(map[string]DownstreamCommandHandler),
		Channels:            make(map[string]*models.ChannelState),
		Users:               make(map[string]*models.UserState),
		ServerName:          "bnc.jordynsblog.org",
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

	reuseMsg := ircmsg.MakeMessage(nil, "", "PRIVMSG", "", "")

	for chatmsg := range msgChan {
		reuseMsg.Source = chatmsg.Source
		reuseMsg.Command = chatmsg.Command // Keep chan notices

		// Overwrite params
		reuseMsg.Params[0] = chatmsg.Target
		reuseMsg.Params[1] = chatmsg.Content

		// Add server-time
		if ds.Caps["server-time"] {
			// Format UNIX epoch as ISO time as IRCv3 dictates
			timeStr := time.Unix(chatmsg.Timestamp, 0).Format(time.RFC3339Nano)
			reuseMsg.SetTag("time", timeStr)
		}

		// If Write fails (e.g., broken pipe), initiate the cleanup
		// which will fire ds.Cancel() and kill the DB query.
		err := ds.SendToClient(reuseMsg)
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

	// Same thing with MODE
	if msg.Command == "MODE" {
		log.Debug().Msgf("[downstream %s] Spoofing prefix for command %s to bouncer server name %s", ds.Conn.RemoteAddr(), msg.Command, b.ServerName)
		msg.Source = b.ServerName
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
	// Create channel state if this is a new channel
	// This is bare now because it is filled in by the mode and topic
	b.mu.Lock()
	if _, exists := b.Channels[channel]; !exists {
		b.Channels[channel] = &models.ChannelState{
			Name: channel,
		}
	}
	b.mu.Unlock()

	b.user_mu.Lock()
	defer b.user_mu.Unlock()

	// Add user state
	for _, rawUser := range users {
		// Check for bugs
		if rawUser == "" {
			BUG(fmt.Sprintf("[upstream %s] AddChannelUsers called with blank user string in array!", b.GetUpstreamConn().Server))
			continue
		}

		nick, prefix := parsePrefix(rawUser)
		log.Trace().Str("nick", nick).Str("prefix", prefix).Str("upstream", b.GetUpstreamConn().Server).Str("channel", channel).Msg("Adding prefixes")
		// Get or create state for user
		user, exists := b.Users[nick]
		if !exists {
			user = &models.UserState{
				Nickname:     nick,
				ChanPrefixes: make(map[string]string),
			}

			b.Users[nick] = user
		}

		user.ChanPrefixes[channel] = prefix
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

func (b *Bouncer) RemoveUserFromChannel(channel string, nick string) {
	b.user_mu.Lock()
	defer b.user_mu.Unlock()

	user, exists := b.Users[nick]
	if !exists {
		log.Debug().Str("channel", channel).Str("nick", nick).Str("upstream", b.GetUpstreamConn().Server).Msg("Attempted removal on non-existent user")
		return
	}

	// Remove the channel from their prefixes
	delete(user.ChanPrefixes, channel)

	// Memory Cleanup: If they are no longer in ANY channels with the bouncer,
	// delete them from the global map.
	if len(user.ChanPrefixes) == 0 {
		log.Debug().Str("channel", channel).Str("nick", nick).Str("upstream", b.GetUpstreamConn().Server).Msg("Removing UserState for user we no longer share any channels with")
		delete(b.Users, nick)
	}
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
	b.user_mu.Lock()
	defer b.user_mu.Unlock()

	adding := true
	argIndex := 0

	for _, char := range modeStr {
		switch char {
		case '+':
			adding = true
		case '-':
			adding = false
		case 'o', 'v', 'h', 'a', 'q':
			if argIndex >= len(args) {
				continue
			}
			targetNick := args[argIndex]
			argIndex++

			user, exists := b.Users[targetNick]
			if !exists {
				continue // User isn't in our global state
			}

			prefix := modeToPrefix(char)
			curPrefix := user.ChanPrefixes[channel]

			if adding {
				curPrefix = curPrefix + prefix
			} else {
				curPrefix = strings.ReplaceAll(curPrefix, prefix, "")
			}

			user.ChanPrefixes[channel] = curPrefix
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
	b.user_mu.Lock()
	defer b.user_mu.Unlock()

	log.Debug().Msgf("[bouncer] Removed UserState for %s", nick)
	delete(b.Users, nick)
}

// ModifyUser safely fetches or creates a user, then applies custom modifications
// inside a thread-safe lock.
func (b *Bouncer) ModifyUser(nick string, modifier func(user *models.UserState)) {
	b.user_mu.Lock()
	defer b.user_mu.Unlock()

	// Fetch existing user, or initialize a new one if they don't exist
	user, exists := b.Users[nick]
	if !exists {
		log.Debug().Msgf("[upstream %s] Creating new UserState for %s", b.GetUpstreamConn().Server, nick)
		user = &models.UserState{
			Nickname:     nick,
			ChanPrefixes: make(map[string]string),
		}
		b.Users[nick] = user
	}

	// Execute the caller's custom logic directly on the struct pointer
	modifier(user)
}
