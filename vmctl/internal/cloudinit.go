package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const userDataTmpl = `#cloud-config
hostname: {{.Name}}
fqdn: {{.Name}}
manage_etc_hosts: true

users:
  - name: {{.Username}}
    ssh_authorized_keys:
      - {{.SSHKey}}
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    lock_passwd: true

# Disable password auth
ssh_pwauth: false

# Grow root partition to fill disk
growpart:
  mode: auto
  devices: ['/']
resize_rootfs: true

# Mount the host-shared directory.
runcmd:
  - mkdir -p /mnt/host
  - mount -t 9p -o trans=virtio,version=9p2000.L,rw hostshare /mnt/host

mounts:
  - [hostshare, /mnt/host, 9p, "trans=virtio,version=9p2000.L,rw,_netdev,nofail,x-systemd.automount", "0", "0"]
`

const metaDataTmpl = `instance-id: {{.Name}}
local-hostname: {{.Name}}
`

type cloudInitData struct {
	Name     string
	Username string
	SSHKey   string
}

// GenerateSeedISO creates a cloud-init seed ISO in the given output directory.
// It uses a temporary directory for intermediate files and calls genisoimage.
func GenerateSeedISO(tmpDir, outputPath, name, username, sshKey string) error {
	data := cloudInitData{
		Name:     name,
		Username: username,
		SSHKey:   sshKey,
	}

	// Render user-data
	ud, err := renderTemplate("user-data", userDataTmpl, data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "user-data"), ud, 0644); err != nil {
		return err
	}

	// Render meta-data
	md, err := renderTemplate("meta-data", metaDataTmpl, data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "meta-data"), md, 0644); err != nil {
		return err
	}

	// Generate ISO
	_, err = runCmd("genisoimage",
		"-output", outputPath,
		"-volid", "cidata",
		"-joliet", "-rock",
		filepath.Join(tmpDir, "user-data"),
		filepath.Join(tmpDir, "meta-data"),
	)
	if err != nil {
		return fmt.Errorf("genisoimage: %w", err)
	}

	return nil
}

func renderTemplate(name, tmpl string, data interface{}) ([]byte, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
