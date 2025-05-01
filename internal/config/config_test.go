package config

import "testing"

func TestRead(t *testing.T) {
	config, err := Read()
	if err != nil {
		t.Errorf("Error reading config: %v", err)
		return
	}
	if config.DbUrl != "postgres://example" {
		t.Errorf("Expected config.DbUrl: %v; Actual: %v", "postgres://example", config.DbUrl)
		return
	}
}

func TestWrite(t *testing.T) {
	config, err := Read()
	if err != nil {
		t.Errorf("Error reading config: %v", err)
		return
	}
	config.SetUser("test_user")
	config2, err := Read()
	if err != nil {
		t.Errorf("Error reading config: %v", err)
		return
	}
	if config2.CurrentUserName != "test_user" {
		t.Errorf("Expected config.CurrentUserName: %v; Actual: %v", "test_user", config2.CurrentUserName)
		return
	}
}
