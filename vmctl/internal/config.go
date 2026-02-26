package internal

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Defaults struct {
	User   string `yaml:"user"`
	VCPUs  int    `yaml:"vcpus"`
	RAMMB  int    `yaml:"ram_mb"`
	DiskGB int    `yaml:"disk_gb"`
}

type Config struct {
	BaseImage string   `yaml:"base_image"`
	Network   string   `yaml:"network"`
	Defaults  Defaults `yaml:"defaults"`
}

func DefaultConfig() *Config {
	return &Config{
		BaseImage: "/mnt/nvme1/vms/base-images/jammy-server-cloudimg-amd64.img",
		Network:   "default",
		Defaults: Defaults{
			User:   "ubuntu",
			VCPUs:  8,
			RAMMB:  16384,
			DiskGB: 100,
		},
	}
}

func LoadConfig() (*Config, error) {
	return LoadConfigForPath("")
}

// LoadConfigForPath loads the global config from ~/.config/vmctl/config.yaml,
// then overlays any config.yaml found in vmPath (if non-empty).
func LoadConfigForPath(vmPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Load global config
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".config", "vmctl", "config.yaml")
		if data, err := os.ReadFile(globalPath); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Overlay per-path config if vmPath is set
	if vmPath != "" {
		localPath := filepath.Join(vmPath, "config.yaml")
		if data, err := os.ReadFile(localPath); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	return cfg, nil
}

// SavePathConfig writes a config.yaml to the given VM path directory.
// It loads any existing per-path config first and merges the updates.
func SavePathConfig(vmPath string, updates *Config) error {
	localPath := filepath.Join(vmPath, "config.yaml")

	// Load existing per-path config to preserve other fields
	existing := &Config{}
	if data, err := os.ReadFile(localPath); err == nil {
		yaml.Unmarshal(data, existing)
	}

	// Apply updates
	if updates.Defaults.VCPUs != 0 {
		existing.Defaults.VCPUs = updates.Defaults.VCPUs
	}
	if updates.Defaults.RAMMB != 0 {
		existing.Defaults.RAMMB = updates.Defaults.RAMMB
	}
	if updates.Defaults.DiskGB != 0 {
		existing.Defaults.DiskGB = updates.Defaults.DiskGB
	}
	if updates.Defaults.User != "" {
		existing.Defaults.User = updates.Defaults.User
	}
	if updates.BaseImage != "" {
		existing.BaseImage = updates.BaseImage
	}
	if updates.Network != "" {
		existing.Network = updates.Network
	}

	data, err := yaml.Marshal(existing)
	if err != nil {
		return err
	}
	return os.WriteFile(localPath, data, 0644)
}
