package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	WorkPath WorkPath `mapstructure:"workpath"`
}

type WorkPath struct {
	Source WorkSource `mapstructure:"source"`
	Tool   string     `mapstructure:"tool"`
	DB     string     `mapstructure:"db"`
}

type WorkSource struct {
	Pic   string `mapstructure:"pic"`
	Video string `mapstructure:"video"`
	Sound string `mapstructure:"sound"`
}

func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tac", "agent")
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetDefault("workpath.source.pic", "./source/pic")
	v.SetDefault("workpath.source.video", "./source/video")
	v.SetDefault("workpath.source.sound", "./source/sound")
	v.SetDefault("workpath.tool", "./tools")
	v.SetDefault("workpath.db", "./data/agent.db")

	v.SetEnvPrefix("TAC")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if configPath != "" {

		v.SetConfigFile(configPath)
	} else {

		v.SetConfigName("properties")
		v.SetConfigType("yaml")
		v.AddConfigPath(DefaultConfigDir())
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/tac")
	}

	if err := v.ReadInConfig(); err != nil {

		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func LoadEnv(envPath string) (map[string]string, error) {
	if envPath == "" {
		envPath = filepath.Join(DefaultConfigDir(), ".env")
	}

	result := make(map[string]string)

	for _, key := range []string{"encryption_key"} {
		if val := os.Getenv("TAC_" + strings.ToUpper(key)); val != "" {
			result[key] = val
		}
	}

	if data, err := os.ReadFile(envPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				if _, exists := result[key]; !exists {
					result[key] = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return result, nil
}

func WriteDefaultConfig(path string) error {
	if path == "" {
		path = filepath.Join(DefaultConfigDir(), "properties.yaml")
	}
	defaultConfig := `workpath:
  source:
    pic: ./source/pic
    video: ./source/video
    sound: ./source/sound
  tool: ./tools
  db: ./data/agent.db
`
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultConfig), 0644)
}

func WriteDefaultEnv(path string) error {
	if path == "" {
		path = filepath.Join(DefaultConfigDir(), ".env")
	}
	defaultEnv := `encryption_key=change-me-to-a-random-string-at-least-32-characters-long
`
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultEnv), 0644)
}
