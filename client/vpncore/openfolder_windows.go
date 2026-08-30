//go:build windows

package vpncore

import (
	"os"

	"golang.org/x/sys/windows"
)

// OpenLogFolder shows the log directory in Explorer.
// Must not use HiddenCommand — CREATE_NO_WINDOW swallows the Explorer window.
func OpenLogFolder() error {
	dir := LogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, path, nil, nil, windows.SW_SHOWNORMAL)
}
