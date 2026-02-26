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
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	path := filepath.Join(home, ".config", "vmctl", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		// Missing config file is fine, use defaults
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
