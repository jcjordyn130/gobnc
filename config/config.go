package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	UpstreamServer   string `mapstructure:"upstream_server"`
	UpstreamPort     int    `mapstructure:"upstream_port"`
	UpstreamPassword string `mapstructure:"upstream_password"`
	UseTLS           bool   `mapstructure:"usetls"`
	IgnoreCerts      bool   `mapstructure:"ignorecerts"`
	Nick             string `mapstructure:"nick"`
	Password         string `mapstructure:"password"`
	BindAddress      string `mapstructure:"bind_address"`
	Verbose          bool   `mapstructure:"verbose"`
	DBPath           string `mapstructure:"dbpath"`
	MaxQLen          int    `mapstructure:"maxqlen"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()

	// 1. Setup file reading
	v.SetConfigName("config") // name of config file (without extension)
	v.SetConfigType("toml")   // REQUIRED if the config file does not have the extension in the name

	// Add config paths
	// TODO: maybe define xdg directory?
	// for now, $PWD is fine for debug
	v.AddConfigPath(".") // look for config in the working directory

	// 2. Setup environment variable overrides
	// If you have a setting named 'upstream_server',
	// viper will look for an environment variable named 'BNC_UPSTREAM_SERVER'
	v.SetEnvPrefix("GOBNC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 3. Set Defaults
	v.SetDefault("upstream_port", 6697)
	v.SetDefault("bind_address", "127.0.0.1:12345")
	v.SetDefault("verbose", true)
	v.SetDefault("MaxQLen", 5000)

	// 4. Read the file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err // Config file found but had an error
		}
		log.Println("No config file found, using defaults and environment variables")
	}

	// 5. Unmarshal into struct
	var conf Config
	if err := v.Unmarshal(&conf); err != nil {
		return nil, err
	}

	return &conf, nil
}
