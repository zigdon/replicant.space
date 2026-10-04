package cfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	cfgFile     = ".config.yaml"
	testCfgFile = "testconfig.yaml"
)

type Config struct {
	APIKey   string `yaml:"api_key"`
	Username string `yaml:"username,omitempty"`
	DBHost   string `yaml:"db_host"`
	DBName   string `yaml:"db_name"`
}

func ReadCfg() (*Config, error) {
	return ReadCfgFile(cfgFile)
}

func ReadTestCfg() (*Config, error) {
	paths := []string{testCfgFile, "../" + testCfgFile, "../../" + testCfgFile}
	for _, p := range paths {
		if c, err := ReadCfgFile(p); err == nil {
			return c, nil
		}
	}
	return ReadCfgFile(testCfgFile)
}

func ReadCfgFile(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("can't read cfg %q: %v", file, err)
	}
	var cfg *Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return cfg, nil
}

func UpdateCfg(newCfg *Config) error {
	data, err := yaml.Marshal(newCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgFile, data, 0755); err != nil {
		return err
	}
	return nil
}
