package internal

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

const libvirtURI = "qemu:///system"

var ipRegexp = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)

// withConnectURI prepends --connect qemu:///system for virsh and virt-install commands.
func withConnectURI(name string, args []string) (string, []string) {
	if name == "virsh" || name == "virt-install" {
		args = append([]string{"--connect", libvirtURI}, args...)
	}
	return name, args
}

func runCmd(name string, args ...string) (string, error) {
	name, args = withConnectURI(name, args)
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func runCmdStdin(stdin, name string, args ...string) (string, error) {
	name, args = withConnectURI(name, args)
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// DomInfo checks if a VM exists. Returns nil if it does, error otherwise.
func DomInfo(name string) error {
	_, err := runCmd("virsh", "dominfo", name)
	return err
}

// DomState returns the current state of a VM (e.g. "running", "shut off").
func DomState(name string) (string, error) {
	out, err := runCmd("virsh", "domstate", name)
	if err != nil {
		return "", err
	}
	return out, nil
}

// DomIfAddr returns the first IP address of a VM, or empty string if none found.
func DomIfAddr(name string) (string, error) {
	out, err := runCmd("virsh", "domifaddr", name)
	if err != nil {
		return "", nil // not an error if no IP yet
	}
	match := ipRegexp.FindString(out)
	return match, nil
}

// Define registers a VM from an XML file.
func Define(xmlPath string) error {
	_, err := runCmd("virsh", "define", xmlPath)
	return err
}

// Start starts a defined VM.
func Start(name string) error {
	_, err := runCmd("virsh", "start", name)
	return err
}

// Shutdown requests a graceful shutdown.
func Shutdown(name string) error {
	_, err := runCmd("virsh", "shutdown", name)
	return err
}

// Destroy forcefully stops a VM. Ignores errors (VM may already be stopped).
func Destroy(name string) {
	runCmd("virsh", "destroy", name)
}

// Undefine removes the VM definition. Ignores errors.
func Undefine(name string) {
	runCmd("virsh", "undefine", name)
}

// DumpXML returns the full XML definition of a VM.
func DumpXML(name string) (string, error) {
	return runCmd("virsh", "dumpxml", name)
}

// AttachDevice attaches a device defined by XML content to a VM (persistent).
func AttachDevice(name, xmlContent string) error {
	_, err := runCmdStdin(xmlContent, "virsh", "attach-device", name, "--config", "/dev/stdin")
	return err
}

// SetVCPUs sets the vCPU count in the persistent VM config (takes effect on next boot).
func SetVCPUs(name string, count int) error {
	if _, err := runCmd("virsh", "setvcpus", name, fmt.Sprintf("%d", count), "--config", "--maximum"); err != nil {
		return err
	}
	_, err := runCmd("virsh", "setvcpus", name, fmt.Sprintf("%d", count), "--config")
	return err
}

// SetMemory sets the memory in the persistent VM config (takes effect on next boot).
// sizeKiB is the memory size in KiB.
func SetMemory(name string, sizeKiB int) error {
	if _, err := runCmd("virsh", "setmaxmem", name, fmt.Sprintf("%d", sizeKiB), "--config"); err != nil {
		return err
	}
	_, err := runCmd("virsh", "setmem", name, fmt.Sprintf("%d", sizeKiB), "--config")
	return err
}

// VirtInstallPrintXML generates the domain XML without creating the VM.
func VirtInstallPrintXML(name string, vcpus, ramMB int, diskPath, isoPath, network, dataDir string) (string, error) {
	return runCmd("virt-install",
		"--name", name,
		"--vcpus", fmt.Sprintf("%d", vcpus),
		"--memory", fmt.Sprintf("%d", ramMB),
		"--disk", fmt.Sprintf("%s,format=qcow2", diskPath),
		"--disk", fmt.Sprintf("%s,device=cdrom,readonly=on", isoPath),
		"--os-variant", "ubuntu22.04",
		"--network", fmt.Sprintf("network=%s", network),
		"--import",
		"--filesystem", fmt.Sprintf("%s,hostshare,type=mount,accessmode=mapped", dataDir),
		"--print-xml",
	)
}

// QemuImgCreate creates a qcow2 image backed by a base image.
func QemuImgCreate(backing, dest string, sizeGB int) error {
	_, err := runCmd("qemu-img", "create", "-f", "qcow2", "-b", backing, "-F", "qcow2", dest, fmt.Sprintf("%dG", sizeGB))
	return err
}
