package cmd

import (
	"fmt"
	"path/filepath"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var resizeCmd = &cobra.Command{
	Use:   "resize <vm-path>",
	Short: "Resize a VM's CPU and/or memory",
	Long: `Update the vCPU count and/or memory for a VM. Changes are saved to
the VM path's config.yaml and applied to the libvirt config.

A VM restart is required for changes to take effect.`,
	Example: `  vmctl resize ~/my-vm -c 4 -m 8
  vmctl resize ~/my-vm --vcpus 16 --memory 32`,
	Args: cobra.ExactArgs(1),
	RunE: runResize,
}

func init() {
	f := resizeCmd.Flags()
	f.IntP("vcpus", "c", 0, "Number of vCPUs")
	f.IntP("memory", "m", 0, "Memory in GB")

	rootCmd.AddCommand(resizeCmd)
}

func runResize(cmd *cobra.Command, args []string) error {
	vmPath, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	vcpus, _ := cmd.Flags().GetInt("vcpus")
	memGB, _ := cmd.Flags().GetInt("memory")

	if vcpus == 0 && memGB == 0 {
		return fmt.Errorf("specify at least one of --vcpus (-c) or --memory (-m)")
	}

	name, _, err := internal.ReadVMInfo(vmPath)
	if err != nil {
		return err
	}

	if err := internal.DomInfo(name); err != nil {
		return fmt.Errorf("VM '%s' not found", name)
	}

	// Apply changes to libvirt config
	if vcpus > 0 {
		fmt.Printf("==> Setting vCPUs to %d...\n", vcpus)
		if err := internal.SetVCPUs(name, vcpus); err != nil {
			return fmt.Errorf("setting vCPUs: %w", err)
		}
	}

	if memGB > 0 {
		memKiB := memGB * 1024 * 1024
		fmt.Printf("==> Setting memory to %d GB...\n", memGB)
		if err := internal.SetMemory(name, memKiB); err != nil {
			return fmt.Errorf("setting memory: %w", err)
		}
	}

	// Save to per-path config.yaml
	updates := &internal.Config{}
	if vcpus > 0 {
		updates.Defaults.VCPUs = vcpus
	}
	if memGB > 0 {
		updates.Defaults.RAMMB = memGB * 1024
	}
	if err := internal.SavePathConfig(vmPath, updates); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("==> Config saved to %s/config.yaml\n", vmPath)
	fmt.Printf("\n    Restart the VM for changes to take effect:\n")
	fmt.Printf("    vmctl shutdown %s && vmctl start %s\n", vmPath, vmPath)

	return nil
}
