package config

import (
	"os"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Host:     "192.168.1.100",
				Username: "admin",
				Port:     22,
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: Config{
				Username: "admin",
				Port:     22,
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: Config{
				Host: "192.168.1.100",
				Port: 22,
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: Config{
				Host:     "192.168.1.100",
				Username: "admin",
				Port:     -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{
		Host:     "192.168.1.100",
		Username: "admin",
	}

	cfg.SetDefaults()

	if cfg.Port != 22 {
		t.Errorf("Expected default port 22, got %d", cfg.Port)
	}
}

func TestConfigMerge(t *testing.T) {
	base := Config{
		Host:     "192.168.1.100",
		Username: "admin",
		Port:     22,
		KeyFile:  "~/.ssh/id_rsa",
	}

	override := Config{
		Host: "192.168.1.200",
		Port: 2222,
	}

	merged := base.MergeWith(override)

	// Should use overridden values
	if merged.Host != "192.168.1.200" {
		t.Errorf("Expected merged host 192.168.1.200, got %s", merged.Host)
	}
	if merged.Port != 2222 {
		t.Errorf("Expected merged port 2222, got %d", merged.Port)
	}

	// Should keep original values where not overridden
	if merged.Username != "admin" {
		t.Errorf("Expected merged username admin, got %s", merged.Username)
	}
	if merged.KeyFile != "~/.ssh/id_rsa" {
		t.Errorf("Expected merged keyfile ~/.ssh/id_rsa, got %s", merged.KeyFile)
	}
}

func TestConfigFileOperations(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "qnap-vm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: failed to cleanup temp dir: %v", err)
		}
	}()

	// Override config path for test
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set HOME env var: %v", err)
	}
	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Logf("Warning: failed to restore HOME env var: %v", err)
		}
	}()

	// Create config file
	configFile := &ConfigFile{
		DefaultHost: "default",
		Hosts: map[string]Config{
			"default": {
				Host:     "192.168.1.100",
				Username: "admin",
				Port:     22,
			},
		},
	}

	// Test saving config
	if err := SaveConfig(configFile); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test loading config
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedConfig.DefaultHost != "default" {
		t.Errorf("Expected default host 'default', got '%s'", loadedConfig.DefaultHost)
	}

	if host, exists := loadedConfig.GetHostConfig("default"); !exists {
		t.Error("Expected to find default host config")
	} else if host.Host != "192.168.1.100" {
		t.Errorf("Expected host 192.168.1.100, got %s", host.Host)
	}
}
