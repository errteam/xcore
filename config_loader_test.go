package xcore

import (
	"os"
	"testing"

	"github.com/fsnotify/fsnotify"
)

type testConfig struct {
	Server struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	} `mapstructure:"server"`
	Database struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"database"`
}

func TestNewConfigLoader(t *testing.T) {
	cl := NewConfigLoader()
	if cl == nil {
		t.Error("NewConfigLoader returned nil")
	}
	if cl.viper == nil {
		t.Error("viper instance is nil")
	}
}

func TestConfigLoader_SetConfigFile(t *testing.T) {
	cl := NewConfigLoader()
	result := cl.SetConfigFile("config.yaml")
	if result != cl {
		t.Error("SetConfigFile should return self for chaining")
	}
}

func TestConfigLoader_SetConfigType(t *testing.T) {
	cl := NewConfigLoader()
	result := cl.SetConfigType("yaml")
	if result != cl {
		t.Error("SetConfigType should return self for chaining")
	}
}

func TestConfigLoader_AddConfigPath(t *testing.T) {
	cl := NewConfigLoader()
	result := cl.AddConfigPath(".")
	if result != cl {
		t.Error("AddConfigPath should return self for chaining")
	}
}

func TestConfigLoader_SetEnvPrefix(t *testing.T) {
	cl := NewConfigLoader()
	result := cl.SetEnvPrefix("APP")
	if result != cl {
		t.Error("SetEnvPrefix should return self for chaining")
	}
}

func TestConfigLoader_AutomaticEnv(t *testing.T) {
	cl := NewConfigLoader()
	result := cl.AutomaticEnv()
	if result != cl {
		t.Error("AutomaticEnv should return self for chaining")
	}
}

func TestConfigLoader_SetEnvKeyReplacer(t *testing.T) {
	cl := NewConfigLoader()
	replacer := cl.SetEnvKeyReplacer("old", "new")
	if replacer == nil {
		t.Error("SetEnvKeyReplacer returned nil")
	}
}

func TestConfigLoader_MergeConfigOverride(t *testing.T) {
	cl := NewConfigLoader()
	err := cl.MergeConfigOverride(map[string]interface{}{
		"key": "value",
	})
	if err != nil {
		t.Errorf("MergeConfigOverride failed: %v", err)
	}
}

func TestConfigLoader_GetString(t *testing.T) {
	cl := NewConfigLoader()
	cl.viper.Set("test.key", "test_value")
	if cl.GetString("test.key") != "test_value" {
		t.Error("GetString returned wrong value")
	}
}

func TestConfigLoader_GetInt(t *testing.T) {
	cl := NewConfigLoader()
	cl.viper.Set("test.key", 123)
	if cl.GetInt("test.key") != 123 {
		t.Error("GetInt returned wrong value")
	}
}

func TestConfigLoader_GetBool(t *testing.T) {
	cl := NewConfigLoader()
	cl.viper.Set("test.key", true)
	if cl.GetBool("test.key") != true {
		t.Error("GetBool returned wrong value")
	}
}

func TestConfigLoader_GetStringSlice(t *testing.T) {
	cl := NewConfigLoader()
	cl.viper.Set("test.key", []string{"a", "b", "c"})
	slice := cl.GetStringSlice("test.key")
	if len(slice) != 3 {
		t.Errorf("expected 3 elements, got %d", len(slice))
	}
}

func TestConfigLoader_Load_NoConfig(t *testing.T) {
	cl := NewConfigLoader()
	err := cl.Load(&testConfig{})
	if err == nil {
		t.Error("Load should fail without config file")
	}
}

func TestConfigLoader_LoadStrict(t *testing.T) {
	cl := NewConfigLoader()
	err := cl.LoadStrict(&testConfig{})
	if err == nil {
		t.Error("LoadStrict should fail without config file")
	}
}

func TestLoadConfigFromFile_NoFile(t *testing.T) {
	err := LoadConfigFromFile("nonexistent.yaml", &testConfig{})
	if err == nil {
		t.Error("LoadConfigFromFile should fail for nonexistent file")
	}
}

func TestLoadConfigFromFiles_NoPaths(t *testing.T) {
	err := LoadConfigFromFiles([]string{}, &testConfig{})
	if err != nil {
		t.Errorf("LoadConfigFromFiles should not fail with empty paths, got: %v", err)
	}
}

func TestLoadConfigFromFiles_NoValidFile(t *testing.T) {
	err := LoadConfigFromFiles([]string{"/nonexistent/config.yaml"}, &testConfig{})
	if err == nil {
		t.Error("LoadConfigFromFiles should fail when no valid config file found")
	}
	if err.Error() != "no valid config file found in provided paths" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadConfigFromFiles_ValidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Skipf("could not create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(`
server:
  port: 8080
  host: "localhost"
database:
  host: "db.local"
  port: 5432
`)
	tmpFile.Close()

	var cfg testConfig
	err = LoadConfigFromFiles([]string{tmpFile.Name()}, &cfg)
	if err != nil {
		t.Errorf("LoadConfigFromFiles failed: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Server.Host)
	}
}

func TestLoadEnvConfig(t *testing.T) {
	os.Setenv("TEST_PORT", "9090")
	os.Setenv("TEST_HOST", "testhost")
	defer os.Unsetenv("TEST_PORT")
	defer os.Unsetenv("TEST_HOST")

	type envConfig struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	}

	err := LoadEnvConfig("TEST", &envConfig{})
	if err != nil {
		t.Errorf("LoadEnvConfig failed: %v", err)
	}
}

func TestConfigLoader_WatchConfig(t *testing.T) {
	cl := NewConfigLoader()
	cl.WatchConfig()
}

func TestConfigLoader_OnConfigChange(t *testing.T) {
	cl := NewConfigLoader()
	cl.OnConfigChange(func(e fsnotify.Event) {})
}
