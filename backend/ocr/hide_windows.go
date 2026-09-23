//go:build windows

package ocr

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps the console app we spawn from flashing a console window in
// the GUI build. Same trick as the git subprocesses in backend/indexer.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
