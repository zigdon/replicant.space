package cfg

import (
	"os"
	"testing"
)

func TestConfigReadAndUpdate(t *testing.T) {
	// Backup existing config file if present
	origContent, origErr := os.ReadFile(cfgFile)
	defer func() {
		if origErr == nil {
			_ = os.WriteFile(cfgFile, origContent, 0755)
		} else {
			_ = os.Remove(cfgFile)
		}
	}()

	testConfig := &Config{
		APIKey:   "test-api-key-12345",
		Username: "tester",
		DBHost:   "localhost",
		DBName:   "replicant_test",
	}

	// 1. UpdateCfg
	if err := UpdateCfg(testConfig); err != nil {
		t.Fatalf("UpdateCfg failed: %v", err)
	}

	// 2. ReadCfg
	readCfg, err := ReadCfg()
	if err != nil {
		t.Fatalf("ReadCfg failed: %v", err)
	}

	if readCfg.APIKey != testConfig.APIKey || readCfg.Username != testConfig.Username ||
		readCfg.DBHost != testConfig.DBHost || readCfg.DBName != testConfig.DBName {
		t.Errorf("ReadCfg mismatch: got %+v, expected %+v", readCfg, testConfig)
	}
}
