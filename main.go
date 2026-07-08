package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bouncer/cmd"
	"bouncer/config"
	"bouncer/core"
	"bouncer/database"
	"bouncer/downstreamHandlers"
	handlers "bouncer/downstreamHandlers"

	_ "net/http/pprof" // The blank identifier is required here to register the handlers

	"github.com/ergochat/irc-go/ircevent"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func mainConnect(ctx context.Context, db *database.DB, conf *config.Config) {
	conn := ircevent.Connection{
		Server: fmt.Sprintf("%s:%d", conf.UpstreamServer, conf.UpstreamPort),
		UseTLS: conf.UseTLS,
		Nick:   conf.Nick,
		Debug:  conf.VerboseUpstream,
	}

	if conf.IgnoreCerts {
		log.Debug().Msg("[main] Ignoring TLS errors due to config...")
		conn.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if conf.UpstreamPassword != "" {
		conn.Password = conf.UpstreamPassword
	}

	// Init bouncer
	b := core.NewBouncer(&conn)

	// Assign the DB pointer to avoid copying sync primitives (sync.WaitGroup/noCopy)
	b.DB = db

	// Register downstream handlers
	// These are commands sent by clients connected to us
	b.Register("PING", handlers.HandlePING)
	b.Register("PRIVMSG", handlers.HandlePRIVMSG)
	b.Register("WHO", handlers.HandleWHO)
	b.Register("USERHOST", handlers.HandleUSERHOST)
	b.Register("NICK", handlers.HandleNICK)
	b.Register("JOIN", handlers.HandleJOIN)
	b.Register("QUIT", downstreamHandlers.HandleQUIT)
	b.Register("PART", downstreamHandlers.HandlePART)
	b.Register("AWAY", downstreamHandlers.HandleAWAY)

	// Start FIFO handler
	go listenFIFO(b, conf.FIFOName)

	// Connect to upstream server
	err := b.ConnectToServer(&conn)
	if err != nil {
		panic("failed to connect to server")
	}

	// Start upstream server loop
	log.Debug().Msg("Starting upstream server loop")
	go conn.Loop()

	// Start downstream listener
	b.ListenDownstream(ctx, "127.0.0.1:12345")

	log.Info().Msg("Gracefully shutting down... This may take up to 30 seconds.")

	// Create time based context for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.GracefulShutdownTimeout)*time.Second)
	defer cancel()

	// Create a channel to listen for the shutdown signal
	// Using as a dumb signal, type doesn't matter here
	shutdownChan := make(chan int)

	go func() {
		// Something causes ListenDownsteam to return (most likely our context), shutdown the bouncer
		b.Shutdown()
		close(shutdownChan)
	}()

	select {
	case <-shutdownChan:
		log.Info().Msg("Graceful shutdown complete.")
	case <-shutdownCtx.Done():
		log.Warn().Msg("Timeout reached... forcing all connections closed")
	}

	// Don't forget the database
	log.Debug().Msg("Closing database")
	db.Close()

	log.Debug().Msg("Byebye :)")
}

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

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	// Init database
	db, err := database.NewDB(conf.DBPath, conf.MaxQLen)
	if err != nil {
		panic(err)
	}

	// Create context to safely close all of the goroutines when the program is terminated
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cliCmd := &cli.Command{
		Name:  "gobnc",
		Usage: "IRC bouncer",
		Commands: []*cli.Command{
			{
				Name:  "connect",
				Usage: "connect to the upstream server and start the bouncer",
				Action: func(c context.Context, cmd *cli.Command) error {
					mainConnect(c, db, conf)
					return nil
				},
			},
			cmd.NewConfigCmd(db).Command(),
		},
	}

	if err := cliCmd.Run(ctx, os.Args); err != nil {
		log.Fatal().Msgf("Error running command: %v", err)
	}
}
