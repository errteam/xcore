package config

import (
	"fmt"
	"strings"
	"xcore-example/pkg/xcore"

	"github.com/spf13/viper"
)

type Config struct {
	Server   xcore.ServerConfig   `mapstructure:"server"`
	Database xcore.DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig            `mapstructure:"jwt"`
	Logger   xcore.LoggerConfig   `mapstructure:"logger"`
}

type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	ExpireHour int    `mapstructure:"expire_hour"`
}

func (c *Config) setDefaults() {

}

func Load(configPath string) (*Config, error) {
	// Set config file
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Override with environment variables (optional)
	viper.SetEnvPrefix("APP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	return &cfg, nil
}
