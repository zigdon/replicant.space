package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	cfgFile = ".config.yaml"
)

type Replicant struct {
	id string `yaml:"id"`
	name string `yaml:"name"`
}

type Configuration struct {
	APIKey string `yaml:"api_key"`
	Replicants []Replicant `yaml:"replicants"`
}

func ReadCfg() (*Configuration, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("can't read cfg %q: %v", cfgFile, err)
	}
	var cfg *Configuration
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return cfg, nil
}

