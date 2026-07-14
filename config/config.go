package config

import (
	_ "embed"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	BindAddress             string `mapstructure:"bind_address"`
	DBPath                  string `mapstructure:"dbpath"`
	MaxQLen                 int    `mapstructure:"maxqlen"`
	GracefulShutdownTimeout int    `mapstructure:"graceful_shutdown_timeout"`
	FIFOName                string `mapstructure:"fifoname"`
	LogLevel                string `mapstructure:"LogLevel"`
}

//go:embed config_example.toml
var DefaultConfig string

func LoadConfig(conf *Config, overrides map[string]any) error {
	v := viper.New()

	// 1. Setup file reading
	v.SetConfigName("config") // name of config file (without extension)
	v.SetConfigType("toml")   // REQUIRED if the config file does not have the extension in the name

	// Add config paths
	// TODO: maybe define xdg directory?
	// for now, $PWD is fine for debug
	v.AddConfigPath(".") // look for config in the working directory

	// 3. Set Defaults
	v.SetDefault("upstream_port", 6697)
	v.SetDefault("bind_address", "127.0.0.1:12345")
	v.SetDefault("MaxQLen", 5000)
	v.SetDefault("fifoname", "")
	v.SetDefault("graceful_shutdown_timeout", 30)

	// Inject any overrides for the CLI
	for k, val := range overrides {
		log.Debug().Msgf("Setting override %s=%s in config!", k, val)
		v.Set(k, val)
	}

	// 4. Read the file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err // Config file found but had an error
		}
		log.Debug().Msg("No config file found, using defaults and environment variables")
	}

	// 5. Unmarshal into struct
	if err := v.Unmarshal(&conf); err != nil {
		return err
	}

	log.Trace().Msgf("Config structure: %+v", conf)

	return nil
}
