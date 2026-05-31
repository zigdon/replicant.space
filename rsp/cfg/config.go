package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	cfgFile = ".config.yaml"
)

type Config struct {
	APIKey string `yaml:"api_key"`
	Username string `yaml:"username"`
}

func ReadCfg() (*Config, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("can't read cfg %q: %v", cfgFile, err)
	}
	var cfg *Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return cfg, nil
}

