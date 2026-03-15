package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// RootConfig defines a mounted directory root.
type RootConfig struct {
	Name string `mapstructure:"name"`
	Path string `mapstructure:"path"`
}

// Config holds the application configuration.
type Config struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	Cache struct {
		Dir string `mapstructure:"dir"`
	} `mapstructure:"cache"`
	Roots []RootConfig `mapstructure:"roots"`
}

// Load reads configuration from file and environment.
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/filemanager")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("cache.dir", ".cache")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if len(cfg.Roots) == 0 {
		return nil, fmt.Errorf("no roots configured")
	}

	return &cfg, nil
}
