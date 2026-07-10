//go:build windows

package main

import (
	"bouncer/core"

	"github.com/rs/zerolog/log"
)

func listenFIFO(_ *core.Bouncer, _ string) {
	log.Error().Msg("FIFOs are NOT supported on Win32")
}
