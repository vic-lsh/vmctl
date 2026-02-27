package cmd

import (
	"fmt"
	"path/filepath"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <vm-path>",
	Short: "Start a stopped VM",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
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
	if state == "running" {
		return fmt.Errorf("VM '%s' is already running", name)
	}

	fmt.Printf("==> Starting VM: %s...\n", name)
	if err := internal.Start(name); err != nil {
		return fmt.Errorf("starting VM: %w", err)
	}

	fmt.Printf("==> VM '%s' started.\n", name)
	return nil
}
