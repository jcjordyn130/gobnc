package main

import (
	"context"
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
	"bouncer/ircevent"

	_ "net/http/pprof" // The blank identifier is required here to register the handlers

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func oldmainConnect(ctx context.Context, db *database.DB, conf *config.Config) {
	// Create config
	usConf := ircevent.NewConfig()
	usConf.Server = conf.UpstreamServer
	usConf.Port = fmt.Sprint(conf.UpstreamPort)
	log.Trace().Msgf("%+v", usConf)

	// Create upstream server
	usServ := ircevent.NewConnection(ctx, usConf)

	// connect
	err := usServ.Connect()
	if err != nil {
		panic(err)
	}

	// Init bouncer
	b := core.NewBouncer(usServ)

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

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create context to safely close all of the goroutines when the program is terminated
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create variables
	// WARN: *technically* this is leaving pointers to unallocated memory in DB, but this is just a placeholder
	// so the program can compile. It will very quickly be overwritten with a valid structure by the Before() function
	//
	// It is fine for Config because all it holds is basic types anyways, so they'll all be blank.
	var conf *config.Config = &config.Config{}
	var db *database.DB = &database.DB{}

	cliCmd := &cli.Command{
		Name:  "gobnc",
		Usage: "IRC bouncer",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "dbpath",
				Aliases:     []string{"db"},
				Usage:       "save database to `file`",
				DefaultText: ":memory:",
			},
			&cli.StringFlag{
				Name:        "loglevel",
				Aliases:     []string{"l"},
				Usage:       "Max log level",
				DefaultText: "trace",
			},
		},

		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			// Process log level
			levelStr := cmd.String("loglevel")

			// This is because levelStr is blank by default
			// but zerolog disables logging for a blank level string.
			if levelStr != "" {
				level, err := zerolog.ParseLevel(levelStr)
				if err != nil {
					return ctx, fmt.Errorf("invalid log level '%s': %w", levelStr, err)
				}

				zerolog.SetGlobalLevel(level)
			}

			// Setup CLI config overrides
			// Add more as needed
			overrides := make(map[string]any)
			if cmd.IsSet("dbpath") {
				overrides["DBPath"] = cmd.String("dbpath")
			}

			// The reason we load config/database here instead of outside of CLI parsing
			// Is so the log level argument affects the two classes while allowing them to be used
			// by CLI commands.
			// Load config
			err := config.LoadConfig(conf, overrides)
			if err != nil {
				panic(err)
			}

			// Init database
			newdb, err := database.NewDB(conf.DBPath, conf.MaxQLen)
			if err != nil {
				panic(err)
			}

			// Override existing pointer for sub commands
			// This *has* to happen otherwise the subcommands still point to a nil DB structure
			// and will cause a segfault.
			*db = *newdb

			return ctx, nil
		},

		Commands: []*cli.Command{
			{
				Name:  "connect",
				Usage: "connect to the upstream server and start the bouncer",
				Action: func(c context.Context, cmd *cli.Command) error {
					mainConnect(c, db, conf)
					return nil
				},
			},
			{
				Name:  "version",
				Usage: "prints the program version",
				Action: func(c context.Context, cmd *cli.Command) error {
					// Override user preference to print the version
					zerolog.SetGlobalLevel(zerolog.InfoLevel)
					log.Info().Msgf("GoBNC version %s", GetVersion())
					return nil
				},
			},
			cmd.NewConfigCmd(db).Command(),
			cmd.NewUserCmd(db).Command(),
			cmd.NewDBCmd(db).Command(),
			cmd.NewServerCmd(db).Command(),
		},
	}

	if err := cliCmd.Run(ctx, os.Args); err != nil {
		log.Fatal().Msgf("Error running command: %v", err)
	}
}
