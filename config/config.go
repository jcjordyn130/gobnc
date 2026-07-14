package config

import (
	_ "embed"
	"sync"

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

var (
	instance *Config
	once     sync.Once
	initErr  error
)

// Load takes a mapping of Config keys to values to override
// These override ALL other configuration sources
func Load(filepath string, overrides map[string]any) error {
	once.Do(func() {
		var conf = Config{}
		v := viper.New()

		if filepath != "" {
			log.Debug().Msgf("Using config file at '%s'", filepath)
			v.SetConfigFile(filepath)
		} else {
			log.Debug().Msgf("No config path given, using config.toml in the current directoy")
			v.SetConfigName("config") // name of config file (without extension)
			v.SetConfigType("toml")   // REQUIRED if the config file does not have the extension in the name

			// Add config paths
			// TODO: maybe define xdg directory?
			// for now, $PWD is fine for debug
			v.AddConfigPath(".") // look for config in the working directory
		}

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
				initErr = err // Config file found but had an error
				return
			}
			log.Debug().Msg("No config file found, using defaults and environment variables")
		}

		// 5. Unmarshal into struct
		if err := v.Unmarshal(&conf); err != nil {
			initErr = err
			return
		}

		log.Trace().Msgf("Config structure: %+v", conf)
		instance = &conf
	})

	return initErr
}

func Get() *Config {
	if instance == nil {
		panic("config.get called without loading first")
	}

	return instance
}
