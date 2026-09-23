//go:build !windows

package ocr

import "os/exec"

// hideWindow is a no-op: Unix GUI apps do not spawn visible consoles.
func hideWindow(cmd *exec.Cmd) {}
