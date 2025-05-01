package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DbUrl           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func Read() (Config, error) {
	var config Config
	homedir, err := os.UserHomeDir()
	if err != nil {
		return config, fmt.Errorf("error reading config: %w", err)
	}

	filepath := filepath.Join(homedir, ".gatorconfig.json")
	data, err := os.ReadFile(filepath)
	if err != nil {
		return config, fmt.Errorf("error reading config: %w", err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("error reading config: %w", err)
	}

	return config, nil
}

func (config Config) SetUser(username string) error {
	config.CurrentUserName = username
	homedir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error writing config: %w", err)
	}
	filepath := filepath.Join(homedir, ".gatorconfig.json")
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error writing config: %w", err)
	}
	err = os.WriteFile(filepath, data, 0666)
	if err != nil {
		return fmt.Errorf("error writing config: %w", err)
	}
	return nil
}
