//go:build windows

package ui

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
)

func tryInstanceLock(path string) (func(), bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	ov := &windows.Overlapped{}
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ov)
	if err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ov); f.Close() }, true, nil
}
