package xcore

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type ConfigLoader struct {
	viper *viper.Viper
}

func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		viper: viper.New(),
	}
}

func (cl *ConfigLoader) SetConfigFile(path string) *ConfigLoader {
	cl.viper.SetConfigFile(path)
	return cl
}

func (cl *ConfigLoader) SetConfigType(typ string) *ConfigLoader {
	cl.viper.SetConfigType(typ)
	return cl
}

func (cl *ConfigLoader) AddConfigPath(path string) *ConfigLoader {
	cl.viper.AddConfigPath(path)
	return cl
}

func (cl *ConfigLoader) SetEnvPrefix(prefix string) *ConfigLoader {
	cl.viper.SetEnvPrefix(prefix)
	return cl
}

func (cl *ConfigLoader) AutomaticEnv() *ConfigLoader {
	cl.viper.AutomaticEnv()
	return cl
}

func (cl *ConfigLoader) SetEnvKeyReplacer(oldNew ...string) *strings.Replacer {
	replacer := strings.NewReplacer(oldNew...)
	cl.viper.SetEnvKeyReplacer(replacer)
	return replacer
}

func (cl *ConfigLoader) MergeConfigOverride(override map[string]interface{}) error {
	return cl.viper.MergeConfigMap(override)
}

func (cl *ConfigLoader) Load(cfg interface{}) error {
	if err := cl.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cl.viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return nil
}

func (cl *ConfigLoader) LoadStrict(cfg interface{}) error {
	if err := cl.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cl.viper.UnmarshalExact(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config strictly: %w", err)
	}
	return nil
}

func (cl *ConfigLoader) GetString(key string) string {
	return cl.viper.GetString(key)
}

func (cl *ConfigLoader) GetInt(key string) int {
	return cl.viper.GetInt(key)
}

func (cl *ConfigLoader) GetBool(key string) bool {
	return cl.viper.GetBool(key)
}

func (cl *ConfigLoader) GetStringSlice(key string) []string {
	return cl.viper.GetStringSlice(key)
}

func (cl *ConfigLoader) WatchConfig() {
	cl.viper.WatchConfig()
}

func (cl *ConfigLoader) OnConfigChange(run func(e fsnotify.Event)) {
	cl.viper.OnConfigChange(run)
}

func LoadConfigFromFile(path string, cfg interface{}) error {
	loader := NewConfigLoader().
		SetConfigFile(path).
		AutomaticEnv()
	return loader.Load(cfg)
}

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
