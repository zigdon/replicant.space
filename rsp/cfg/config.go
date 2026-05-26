package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	cfgFile = ".config.yaml"
)

type Configuration struct {
	APIKey string `yaml:"api_key"`
	Username string `yaml:"username"`
	Replicants map[string]string `yaml:"replicants"`
}

func GetID(id int) string {
	config, err := ReadCfg()
	if err != nil {
		panic(fmt.Sprintf("Couldn't read config: %v", err))
	}
	if len(config.Replicants) < id-1 {
		fmt.Printf("Error: Only %d replicants configured, can't find %d", len(config.Replicants), id)
		return ""
	}

	return config.Replicants[fmt.Sprintf("%s-%d", config.Username, id)]
}

var dumped bool

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

	if !dumped {
		dumped = true
		fmt.Printf("config: %#v\n", cfg)
	}

	return cfg, nil
}

