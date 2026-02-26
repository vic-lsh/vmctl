package cmd

import (
	"fmt"
	"path/filepath"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown <vm-path>",
	Short: "Gracefully shut down a VM",
	Args:  cobra.ExactArgs(1),
	RunE:  runShutdown,
}

func init() {
	rootCmd.AddCommand(shutdownCmd)
}

func runShutdown(cmd *cobra.Command, args []string) error {
	vmPath, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	name, _, err := internal.ReadVMInfo(vmPath)
	if err != nil {
		return err
	}

	state, err := internal.DomState(name)
	if err != nil {
		return fmt.Errorf("VM '%s' not found", name)
	}
	if state != "running" {
		return fmt.Errorf("VM '%s' is not running (state: %s)", name, state)
	}

	fmt.Printf("==> Shutting down VM: %s...\n", name)
	if err := internal.Shutdown(name); err != nil {
		return fmt.Errorf("shutting down VM: %w", err)
	}

	fmt.Printf("==> Shutdown signal sent to '%s'.\n", name)
	return nil
}
