// Package xcore provides configuration loading functionality for the xcore framework.
//
// This package provides utilities for loading configuration from various sources:
//   - Files (YAML, JSON, TOML, etc.) using viper
//   - Environment variables with prefix support
//   - Multiple file paths with fallback
//
// The ConfigLoader wraps the viper library to provide a fluent interface
// for configuration management.
package xcore

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ConfigLoader wraps the viper library to provide a fluent configuration loading interface.
// It supports loading from files, environment variables, and can watch for configuration changes.
type ConfigLoader struct {
	viper *viper.Viper
}

// NewConfigLoader creates a new ConfigLoader instance with a fresh viper instance.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		viper: viper.New(),
	}
}

// SetConfigFile sets the configuration file path and type.
// This is used to specify the main configuration file.
func (cl *ConfigLoader) SetConfigFile(path string) *ConfigLoader {
	cl.viper.SetConfigFile(path)
	return cl
}

// SetConfigType sets the configuration file type (e.g., "yaml", "json", "toml").
// This is used when the file extension alone is not sufficient to determine the format.
func (cl *ConfigLoader) SetConfigType(typ string) *ConfigLoader {
	cl.viper.SetConfigType(typ)
	return cl
}

// AddConfigPath adds a directory path to search for configuration files.
// Multiple paths can be added and they are searched in order.
func (cl *ConfigLoader) AddConfigPath(path string) *ConfigLoader {
	cl.viper.AddConfigPath(path)
	return cl
}

// SetEnvPrefix sets the prefix for environment variables.
// Environment variables with this prefix will be mapped to configuration keys.
func (cl *ConfigLoader) SetEnvPrefix(prefix string) *ConfigLoader {
	cl.viper.SetEnvPrefix(prefix)
	return cl
}

// AutomaticEnv enables automatic mapping of environment variables to configuration keys.
// It automatically reads environment variables that match the configuration structure.
func (cl *ConfigLoader) AutomaticEnv() *ConfigLoader {
	cl.viper.AutomaticEnv()
	return cl
}

// SetEnvKeyReplacer sets a replacer for environment variable keys.
// This allows transforming keys (e.g., replacing "-" with ".").
// Returns the created Replacer for reference.
func (cl *ConfigLoader) SetEnvKeyReplacer(oldNew ...string) *strings.Replacer {
	replacer := strings.NewReplacer(oldNew...)
	cl.viper.SetEnvKeyReplacer(replacer)
	return replacer
}

// MergeConfigOverride merges configuration overrides from a map.
// This is useful for programmatically overriding specific configuration values.
func (cl *ConfigLoader) MergeConfigOverride(override map[string]interface{}) error {
	return cl.viper.MergeConfigMap(override)
}

// Load reads the configuration file and unmarshals it into the provided struct.
// It returns an error if the file cannot be read or the content cannot be unmarshaled.
func (cl *ConfigLoader) Load(cfg interface{}) error {
	if err := cl.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cl.viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return nil
}

// LoadStrict reads the configuration file and unmarshals it strictly into the provided struct.
// Strict unmarshaling requires exact field matching and returns an error for unknown fields.
func (cl *ConfigLoader) LoadStrict(cfg interface{}) error {
	if err := cl.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cl.viper.UnmarshalExact(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config strictly: %w", err)
	}
	return nil
}

// GetString returns the string value for the specified key.
// Returns empty string if the key does not exist.
func (cl *ConfigLoader) GetString(key string) string {
	return cl.viper.GetString(key)
}

// GetInt returns the integer value for the specified key.
// Returns 0 if the key does not exist.
func (cl *ConfigLoader) GetInt(key string) int {
	return cl.viper.GetInt(key)
}

// GetBool returns the boolean value for the specified key.
// Returns false if the key does not exist.
func (cl *ConfigLoader) GetBool(key string) bool {
	return cl.viper.GetBool(key)
}

// GetStringSlice returns the string slice value for the specified key.
// Returns nil if the key does not exist.
func (cl *ConfigLoader) GetStringSlice(key string) []string {
	return cl.viper.GetStringSlice(key)
}

// WatchConfig enables watching for configuration file changes.
// When the configuration file changes, it is automatically reloaded.
func (cl *ConfigLoader) WatchConfig() {
	cl.viper.WatchConfig()
}

// OnConfigChange registers a callback function to be called when configuration changes.
// The callback receives the fsnotify.Event describing the change.
func (cl *ConfigLoader) OnConfigChange(run func(e fsnotify.Event)) {
	cl.viper.OnConfigChange(run)
}

// LoadConfigFromFile loads configuration from a single file path.
// It uses the file extension to determine the configuration format.
// Automatically enables environment variable mapping.
func LoadConfigFromFile(path string, cfg interface{}) error {
	loader := NewConfigLoader().
		SetConfigFile(path).
		AutomaticEnv()
	return loader.Load(cfg)
}

// LoadConfigFromFiles loads configuration from multiple file paths.
// It attempts to load the first valid file found in the list.
// Returns an error if no valid config file is found or if loading fails.
func LoadConfigFromFiles(paths []string, cfg interface{}) error {
	loader := NewConfigLoader().
		AddConfigPath(".").
		AutomaticEnv()

	found := false
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			loader.SetConfigFile(path)
			found = true
			break
		}
	}

	if !found && len(paths) > 0 {
		return errors.New("no valid config file found in provided paths")
	}

	return loader.Load(cfg)
}

// LoadEnvConfig loads configuration from environment variables with the given prefix.
// It automatically maps environment variables to configuration keys.
// Keys are transformed by replacing "-" with ".".
func LoadEnvConfig(prefix string, cfg interface{}) error {
	v := viper.New()
	v.SetEnvPrefix(prefix)
	v.AutomaticEnv()
	replacer := strings.NewReplacer("-", ".")
	v.SetEnvKeyReplacer(replacer)

	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal env config: %w", err)
	}
	return nil
}
