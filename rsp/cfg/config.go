package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
    "github.com/zigdon/rsp/models"
)

const (
	cfgFile = ".config.yaml"
)

func ReadCfg() (*models.Config, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("can't read cfg %q: %v", cfgFile, err)
	}
	var cfg *models.Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return cfg, nil
}

