package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the configuration for connecting to a QNAP device
type Config struct {
	Host     string `yaml:"host" json:"host"`
	Username string `yaml:"username" json:"username"`
	Port     int    `yaml:"port" json:"port"`
	KeyFile  string `yaml:"keyfile" json:"keyfile"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// ConfigFile represents the structure of the configuration file
type ConfigFile struct {
	DefaultHost string            `yaml:"default_host" json:"default_host"`
	Hosts       map[string]Config `yaml:"hosts" json:"hosts"`
}

const (
	configDir  = ".qnap-vm"
	configFile = "config.yaml"
)

// GetConfigPath returns the path to the configuration file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, configDir, configFile)
	return configPath, nil
}

// LoadConfig loads configuration from file
func LoadConfig() (*ConfigFile, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Return empty config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &ConfigFile{
			Hosts: make(map[string]Config),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]Config)
	}

	return &cfg, nil
}

// SaveConfig saves configuration to file
func SaveConfig(cfg *ConfigFile) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetHostConfig returns configuration for a specific host
func (cf *ConfigFile) GetHostConfig(hostName string) (Config, bool) {
	if hostName == "" {
		hostName = cf.DefaultHost
	}

	if hostName == "" && len(cf.Hosts) == 1 {
		// If there's only one host configured, use it as default
		for _, config := range cf.Hosts {
			return config, true
		}
	}

	config, exists := cf.Hosts[hostName]
	return config, exists
}

// SetHostConfig sets configuration for a specific host
func (cf *ConfigFile) SetHostConfig(hostName string, config Config) {
	if cf.Hosts == nil {
		cf.Hosts = make(map[string]Config)
	}
	cf.Hosts[hostName] = config
}

// SetDefaultHost sets the default host
func (cf *ConfigFile) SetDefaultHost(hostName string) {
	cf.DefaultHost = hostName
}

// ListHosts returns a list of configured hosts
func (cf *ConfigFile) ListHosts() []string {
	hosts := make([]string, 0, len(cf.Hosts))
	for hostName := range cf.Hosts {
		hosts = append(hosts, hostName)
	}
	return hosts
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.Port)
	}
	return nil
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = 22
	}
}

// MergeWith merges this config with another config, with the other taking precedence
func (c *Config) MergeWith(other Config) Config {
	result := *c

	if other.Host != "" {
		result.Host = other.Host
	}
	if other.Username != "" {
		result.Username = other.Username
	}
	if other.Port != 0 {
		result.Port = other.Port
	}
	if other.KeyFile != "" {
		result.KeyFile = other.KeyFile
	}
	if other.Password != "" {
		result.Password = other.Password
	}

	return result
}
