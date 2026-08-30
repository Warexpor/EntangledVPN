//go:build !windows

package vpncore

import (
	"os"
	"os/exec"
)

func HiddenCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func OpenLogFolder() error {
	dir := LogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return exec.Command("xdg-open", dir).Start()
}
