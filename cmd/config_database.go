package cmd

import (
	"bouncer/database"
	"context"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// 1. Define the single type that holds dependencies for this subcommand domain
type DBCmd struct {
	db *database.DB
	// You could add other specific dependencies here later, like:
	// logger *zap.Logger
}

// 2. Create a constructor to enforce dependency injection
func NewDBCmd(db *database.DB) *DBCmd {
	return &DBCmd{
		db: db,
	}
}

func (c *DBCmd) validateDB(ctx context.Context, cmd *cli.Command) error {
	log.Info().Msg("Checking database...")
	err := c.db.Check()
	if err != nil {
		return err
	}

	log.Info().Msg("Database is all good!")
	return nil
}

func (c *DBCmd) vacDB(ctx context.Context, cmd *cli.Command) error {
	log.Info().Msg("Optimizing database...")
	err := c.db.Optimize()
	if err != nil {
		return err
	}

	log.Info().Msg("Optimization complete.")
	return nil
}

func (c *DBCmd) Command() *cli.Command {
	return &cli.Command{
		Name:  "database",
		Usage: "database operations",
		Commands: []*cli.Command{
			{
				Name:      "validate",
				Usage:     "check database for consistency",
				ArgsUsage: "",
				Action:    c.validateDB,
			},
			{
				Name:      "optimize",
				Usage:     "defragments DB",
				ArgsUsage: "",
				Action:    c.vacDB,
			},
		},
	}
}
