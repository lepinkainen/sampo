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

// DetectionConfig holds person detection settings.
type DetectionConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	ModelPath    string  `mapstructure:"model_path"`
	ModelVersion string  `mapstructure:"model_version"`
	Threshold    float32 `mapstructure:"threshold"`
	Workers      int     `mapstructure:"workers"`
}

// ClassificationConfig holds CLIP classification settings.
type ClassificationConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	ModelPath    string  `mapstructure:"model_path"`
	LabelsPath   string  `mapstructure:"labels_path"`
	ModelVersion string  `mapstructure:"model_version"`
	Threshold    float32 `mapstructure:"threshold"`
	Workers      int     `mapstructure:"workers"`
}

// Config holds the application configuration.
type Config struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	Cache struct {
		Dir string `mapstructure:"dir"`
	} `mapstructure:"cache"`
	Roots          []RootConfig         `mapstructure:"roots"`
	Detection      DetectionConfig      `mapstructure:"detection"`
	Classification ClassificationConfig `mapstructure:"classification"`
}

// Load reads configuration from file and environment.
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/filemanager")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("cache.dir", ".cache")
	viper.SetDefault("detection.enabled", false)
	viper.SetDefault("detection.model_path", "models/yolov8n.onnx")
	viper.SetDefault("detection.model_version", "v8n-1.0")
	viper.SetDefault("detection.threshold", 0.5)
	viper.SetDefault("detection.workers", 2)
	viper.SetDefault("classification.enabled", false)
	viper.SetDefault("classification.model_path", "models/clip-vit-b32-image.onnx")
	viper.SetDefault("classification.labels_path", "models/clip-labels.json")
	viper.SetDefault("classification.model_version", "clip-vit-b32-1.0")
	viper.SetDefault("classification.threshold", 0.2)
	viper.SetDefault("classification.workers", 2)

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
