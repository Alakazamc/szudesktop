package main

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

func startupError(message string) {
	u := windows.NewLazySystemDLL("user32.dll")
	p := u.NewProc("MessageBoxW")
	title, _ := windows.UTF16PtrFromString("szuDesktop")
	text, _ := windows.UTF16PtrFromString(message)
	p.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
