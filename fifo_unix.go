//go:build !windows

package main

import (
	"bouncer/core"
	"bufio"
	"os"
	"syscall"

	"github.com/rs/zerolog/log"
)

func listenFIFO(b *core.Bouncer, fifoName string) {
	if fifoName == "" {
		log.Info().Msg("No FIFO name provided, skipping FIFO listener...")
		return
	}

	// Create the FIFO file if it doesn't already exist
	if _, err := os.Stat(fifoName); os.IsNotExist(err) {
		// 0666 sets read/write permissions for the pipe
		err := syscall.Mkfifo(fifoName, 0666)
		if err != nil {
			log.Fatal().Msgf("Failed to create FIFO: %v", err)
		}
	}

	log.Debug().Msgf("Listening for input on %s...", fifoName)

	// Infinite loop: Required to reopen the FIFO when a writer disconnects
	for {
		// os.OpenFile blocks here until an external process opens the FIFO for writing
		file, err := os.OpenFile(fifoName, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			log.Debug().Msgf("Error opening FIFO: %v", err)
			continue
		}

		// Read lines as they come in
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			log.Debug().Msgf("[FIFO] Received: %s", line)

			// Send the line to the IRC channel
			b.GetUpstreamConn().SendRaw(line)
		}

		if err := scanner.Err(); err != nil {
			log.Debug().Msgf("[FIFO] Error reading from FIFO: %v", err)
		}

		// The writer closed the pipe (EOF). Close our end, loop around, and block again.
		file.Close()
	}
}
