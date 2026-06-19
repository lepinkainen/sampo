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

// OCRConfig holds optical character recognition settings.
//
// The backend is selected at compile time by build tags in internal/ocr:
// macOS shells out to BinaryPath (the Vision-framework `sampo-ocr` helper);
// Linux/other runs in-process ONNX PP-OCR using DetModelPath/RecModelPath/DictPath.
type OCRConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	BinaryPath   string `mapstructure:"binary_path"`    // darwin: sampo-ocr Vision helper
	DetModelPath string `mapstructure:"det_model_path"` // onnx: DBNet detection model
	RecModelPath string `mapstructure:"rec_model_path"` // onnx: CRNN recognition model
	DictPath     string `mapstructure:"dict_path"`      // onnx: recognition char dict
	ModelVersion string `mapstructure:"model_version"`
	Workers      int    `mapstructure:"workers"`
}

// AnalysisConfig holds browse-triggered automatic analysis settings.
type AnalysisConfig struct {
	AutoBrowseEnabled bool `mapstructure:"auto_browse_enabled"`
	BrowseWorkers     int  `mapstructure:"browse_workers"`
	BrowseQueueSize   int  `mapstructure:"browse_queue_size"`
	IncludeVideos     bool `mapstructure:"include_videos"`
}

// Config holds the application configuration.
type Config struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	Cache struct {
		Dir        string `mapstructure:"dir"`
		MaxAgeDays int    `mapstructure:"max_age_days"`
	} `mapstructure:"cache"`
	Roots          []RootConfig         `mapstructure:"roots"`
	Detection      DetectionConfig      `mapstructure:"detection"`
	Classification ClassificationConfig `mapstructure:"classification"`
	OCR            OCRConfig            `mapstructure:"ocr"`
	Analysis       AnalysisConfig       `mapstructure:"analysis"`
}

// Load reads configuration from file and environment.
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/sampo")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("cache.dir", ".cache")
	viper.SetDefault("cache.max_age_days", 90)
	viper.SetDefault("detection.enabled", false)
	viper.SetDefault("detection.model_path", "models/yolo11n.onnx")
	viper.SetDefault("detection.model_version", "v11n-1.0")
	viper.SetDefault("detection.threshold", 0.5)
	viper.SetDefault("detection.workers", 2)
	viper.SetDefault("classification.enabled", false)
	viper.SetDefault("classification.model_path", "models/clip-vit-b32-image.onnx")
	viper.SetDefault("classification.labels_path", "models/clip-labels.json")
	viper.SetDefault("classification.model_version", "clip-vit-b32-1.0")
	viper.SetDefault("classification.threshold", 0.2)
	viper.SetDefault("classification.workers", 2)
	viper.SetDefault("ocr.enabled", false)
	viper.SetDefault("ocr.binary_path", "bin/sampo-ocr")
	viper.SetDefault("ocr.model_version", "vision-1.0")
	viper.SetDefault("ocr.workers", 2)
	viper.SetDefault("analysis.auto_browse_enabled", true)
	viper.SetDefault("analysis.browse_workers", 1)
	viper.SetDefault("analysis.browse_queue_size", 128)
	viper.SetDefault("analysis.include_videos", true)

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
