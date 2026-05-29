package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
    "github.com/zigdon/rsp/models"
)

func log(tmpl string, args ...any) {
	if os.Getenv("DEBUG_CONFIG") != "" {
		fmt.Fprintf(os.Stderr, tmpl, args...)
	}
}

const (
	cfgFile = ".config.yaml"
)

func GetID(id int) string {
	config, err := ReadCfg()
	if err != nil {
		panic(fmt.Sprintf("Couldn't read config: %v", err))
	}
	if len(config.Replicants) < id-1 {
		log("Error: Only %d replicants configured, can't find %d", len(config.Replicants), id)
		return ""
	}

	return config.Replicants[fmt.Sprintf("%s-%d", config.Username, id)]
}

var dumped bool

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

	if !dumped {
		dumped = true
		log("config: %#v\n", cfg)
	}

	return cfg, nil
}

