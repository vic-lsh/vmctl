package cmd

import (
	"fmt"
	"os"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var cfg *internal.Config

var rootCmd = &cobra.Command{
	Use:   "vmctl",
	Short: "Manage KVM/QEMU virtual machines",
	Long: `vmctl provisions and manages KVM/QEMU VMs with cloud-init and 9p shared storage.

Configuration is loaded from ~/.config/vmctl/config.yaml (optional).
CLI flags override config values.`,
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(addShareCmd)
}

func Execute() error {
	var err error
	cfg, err = internal.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = internal.DefaultConfig()
	}
	return rootCmd.Execute()
}

// GetConfig returns the loaded configuration for use by subcommands.
func GetConfig() *internal.Config {
	return cfg
}
