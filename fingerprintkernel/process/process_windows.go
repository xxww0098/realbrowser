//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func ConfigureCommand(cmd *exec.Cmd, mode CommandMode) {
	if mode == DetachedBrowser {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	}
}
func signalTree(pid int, sig syscall.Signal) error {
	args := []string{"/PID", strconv.Itoa(pid), "/T"}
	if sig == syscall.SIGKILL {
		args = append(args, "/F")
	}
	cmd := exec.Command("taskkill", args...)
	ConfigureCommand(cmd, PrivateDriver)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fingerprintkernel: taskkill: %w (%s)", err, out)
	}
	return nil
}
func terminateTree(pid int, grace time.Duration) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	_ = signalTree(pid, syscall.SIGTERM)
	state, err := windows.WaitForSingleObject(h, uint32(min(grace.Milliseconds(), int64(windows.INFINITE-1))))
	if err != nil || state == windows.WAIT_OBJECT_0 {
		return err
	}
	return signalTree(pid, syscall.SIGKILL)
}
