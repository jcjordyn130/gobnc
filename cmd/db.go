package cmd

import (
	"bouncer/config"
	"bouncer/database"
	"context"
	"crypto/rand"
	"fmt"

	"os"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func DBFillFakeData(ctx context.Context) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Init database
	bdb, err := database.NewDB(conf.DBPath, conf.MaxQLen)
	if err != nil {
		panic(err)
	}

	// Generate fake messages
	log.Debug().Msg("Generating 4096 fake messages on 10 channels...")
	for range 10 {
		channelName := "##" + rand.Text()[0:10]
		log.Debug().Msgf("Generating 4096 fake messages for channel %s", channelName)
		for range 4096 {
			// Random chan
			message := ircmsg.MakeMessage(nil, rand.Text()[0:10], "PRIVMSG", channelName, rand.Text())
			bdb.LogQueue <- message

			// Random user DM
			message2 := ircmsg.MakeMessage(nil, rand.Text()[0:10], "PRIVMSG", "jcjordyn130", rand.Text())
			bdb.LogQueue <- message2
		}
	}

	// Add fake messages to existing chans
	for range 4096 {
		message := ircmsg.MakeMessage(nil, rand.Text()[0:10], "PRIVMSG", "##jcj", rand.Text())
		bdb.LogQueue <- message
	}

	for i := range 4 {
		for range 4096 {
			message := ircmsg.MakeMessage(nil, rand.Text()[0:10], "PRIVMSG", "##jcj"+fmt.Sprint(i), rand.Text())
			bdb.LogQueue <- message
		}
	}

	log.Debug().Msg("Done generating fake messages.")

	bdb.Close()

	return nil
}

func main() {
	cmd := &cli.Command{
		Name:  "db",
		Usage: "Database debug functions",
		Commands: []*cli.Command{
			{
				Name:  "fillFakeData",
				Usage: "fill the database with fake data for testing purposes",
				Action: func(c context.Context, cmd *cli.Command) error {
					return DBFillFakeData(c)
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Msgf("Error running command: %v", err)
	}
}
