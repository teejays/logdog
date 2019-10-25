package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/teejays/clog"
)

// Config defines the structure of the configuration file for the application.
type Config struct {
	InQueueBufferSize     int               `toml:"queue_buffer_size"`
	AppLogSupressionLevel int               `toml:"debug_level_not"`
	LogSources            []ConfigLogSource `toml:"log_sources"`
	Stats                 struct {
		Types []ConfigStatsType
	}
	Alert struct {
		Types []ConfigAlertType
	}
}

// ConfigLogSource is information from the config file regarding LogSources that the application need to use.
type ConfigLogSource struct {
	Name     string
	Type     string
	Path     string
	Disabled bool
	Settings ConfigLogSourceSettings
}

type ConfigLogSourceSettings struct {
	Format               string
	Headers              []string
	TimestampKey         string `toml:"timestamp_key"`
	TimestampFormat      string `toml:"timestamp_format"`
	UseFirstlineAsHeader bool   `toml:"use_firstline_as_header"`
}

type ConfigStatsType struct {
	Name            string
	DurationSeconds int64 `toml:"duration_seconds"`
	Disabled        bool
	SourceSettings  []ConfigStatsTypeSourceSetting `toml:"source_settings"`
}

type ConfigStatsTypeSourceSetting struct {
	Name                string
	Key                 string
	ValueMutateFuncName string   `toml:"value_mutator_func"`
	OtherKeys           []string `toml:"other_keys"`
}

type ConfigAlertType struct {
	Name            string
	DurationSeconds int64 `toml:"duration_seconds"`
	Threshold       int
	Disabled        bool
	SourceSettings  []ConfigAlertTypeSourceSetting `toml:"source_settings"`
}

type ConfigAlertTypeSourceSetting struct {
	Name                string
	Key                 string
	ValueMutateFuncName string `toml:"value_mutator_func"`
	Values              []string
}

// ReadConfigTOML takes a path to a config file in TOML format, and parses it into a Config struct
func ReadConfigTOML(path string) (Config, error) {
	var cfg Config

	if strings.TrimSpace(path) == "" {
		return cfg, fmt.Errorf("empty config file path")
	}

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, err
	}

	// Do some validation
	if cfg.InQueueBufferSize < 1 {
		return cfg, fmt.Errorf("InQueueBufferSize has an invalid value: %d", cfg.InQueueBufferSize)
	}

	clog.LogLevel = cfg.AppLogSupressionLevel

	clog.Debugf("Config Log Sources: %v", cfg.LogSources)
	if len(cfg.LogSources) < 1 {
		return cfg, fmt.Errorf("no log source detected from config file")
	}

	return cfg, nil
}
