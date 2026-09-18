//go:build !windows

package netpref

import "os/exec"

func hideProcess(cmd *exec.Cmd) {}
