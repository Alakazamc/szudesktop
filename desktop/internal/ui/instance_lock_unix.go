//go:build !windows

package ui

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func tryInstanceLock(path string) (func(), bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, true, nil
}
