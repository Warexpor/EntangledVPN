//go:build !windows

package vpncore

import "os/exec"

// HiddenCommand is a plain exec.Command on non-Windows (CI stubs).
func HiddenCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
