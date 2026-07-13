package ircevent

import (
	"bufio"
	"context"
	"net"
	"sync"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
	"github.com/rs/zerolog"
)

type UpstreamCommandHandler func(msg ircmsg.Message) (bool, error)

type UpstreamConnection struct {
	Config upstreamConfig

	conn        net.Conn
	writer      *bufio.Writer
	reader      *ircreader.Reader
	msgChan     chan ircmsg.Message
	callbacks   map[string][]UpstreamCommandHandler
	callback_mu sync.RWMutex
	logger      zerolog.Logger

	CurrentCaps []string
	connected   bool
	// These hold the signal for goroutines that are using this
	// to exit on disconnect
	Ctx    context.Context
	Cancel context.CancelFunc
}

// Private because it is NOT intended to be created raw outside of the ircevent library
type upstreamConfig struct {
	Server string
	Port   string

	// Time in ms to wait for new messages before force flushing the buffer
	WriterLoopTimeout int
}
