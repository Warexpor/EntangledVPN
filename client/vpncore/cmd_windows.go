//go:build windows

package vpncore

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// HiddenCommand is exec.Command that does not flash a console window on Windows.
func HiddenCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
