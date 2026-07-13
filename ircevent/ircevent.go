package ircevent

import (
	"context"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func NewConnection(ctx context.Context, config upstreamConfig) *UpstreamConnection {
	us := UpstreamConnection{
		Config:    config,
		connected: false,
	}

	if us.Config.Server == "" || us.Config.Port == "" {
		log.Error().Msgf("Server or Port not set on UpstreamConnection")
		return nil
	}

	// Init logger
	us.logger = log.With().Str("server", us.Config.Server).Str("port", us.Config.Port).Logger()

	// Create writer channel
	us.msgChan = make(chan ircmsg.Message)

	// Init context
	us.Ctx, us.Cancel = context.WithCancel(ctx)

	// Init callback mapping
	us.callbacks = make(map[string][]UpstreamCommandHandler)

	// Init negotiated cap list
	us.CurrentCaps = make([]string, 0)

	// Register internal callbacks
	//us.RegisterCallback("CAP", handleCAP)

	return &us
}

func NewConfig() upstreamConfig {
	// Create raw struc
	usconf := upstreamConfig{}

	// Set default values
	usconf.WriterLoopTimeout = 75

	return usconf
}
