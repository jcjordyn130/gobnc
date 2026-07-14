package cmd

import (
	"bouncer/config"
	"bouncer/database"
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// 1. Define the single type that holds dependencies for this subcommand domain
type ConfigCmd struct {
	db *database.DB
	// You could add other specific dependencies here later, like:
	// logger *zap.Logger
}

// 2. Create a constructor to enforce dependency injection
func NewConfigCmd(db *database.DB) *ConfigCmd {
	return &ConfigCmd{
		db: db,
	}
}

func (c *ConfigCmd) listAutoJoinChans(ctx context.Context, cmd *cli.Command) error {
	chans, err := c.db.GetAutoJoinChans()
	if err != nil {
		return err
	}

	if len(chans) == 0 {
		fmt.Println("No autojoin channels added!")
		return nil
	}

	fmt.Println("------------------")
	fmt.Println("Autojoin Channels:")
	fmt.Println("------------------")
	for _, ch := range chans {
		fmt.Printf("%s\n", ch)
	}

	return nil
}

func (c *ConfigCmd) addAutoJoinChan(ctx context.Context, cmd *cli.Command) error {
	channel := cmd.Args().First()
	if channel == "" {
		return fmt.Errorf("error: channel name is required")
	}

	err := c.db.AddAutoJoinChan(channel)
	if err != nil {
		return err
	}
	return nil
}

func (c *ConfigCmd) removeAutoJoinChan(ctx context.Context, cmd *cli.Command) error {
	channel := cmd.Args().First()
	if channel == "" {
		return fmt.Errorf("error: channel name is required")
	}

	err := c.db.RemoveAutoJoinChan(channel)
	if err != nil {
		return err
	}
	return nil
}

func (c *ConfigCmd) printDefault(ctx context.Context, cmd *cli.Command) error {
	println(config.DefaultConfig)
	return nil
}

func (c *ConfigCmd) Command() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "configure the bouncer",
		Commands: []*cli.Command{
			{
				Name:   "default",
				Usage:  "prints the default configuration file",
				Action: c.printDefault,
			},
			{
				Name:  "autojoin",
				Usage: "manage the autojoin list",
				// Nest the add/remove/list actions under autojoin
				Commands: []*cli.Command{
					{
						Name:      "add",
						Usage:     "add a channel to the autojoin list",
						ArgsUsage: "<channel>",
						Action:    c.addAutoJoinChan,
					},
					{
						Name:      "remove",
						Aliases:   []string{"rm", "del"},
						Usage:     "remove a channel from the autojoin list",
						ArgsUsage: "<channel>",
						Action:    c.removeAutoJoinChan,
					},
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Usage:   "list all autojoin channels",
						Action:  c.listAutoJoinChans,
					},
				},
			},
			// You can easily add other config subcommands here later, e.g.:
			// {
			// 	Name:  "server",
			// 	Usage: "manage upstream server settings",
			// 	// ...
			// },
		},
	}
}
