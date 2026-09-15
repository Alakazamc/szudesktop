//go:build !windows

package main

import "errors"

// 非 Windows 平台还没做开机自启（macOS 要写 LaunchAgent，Linux 要写 systemd user unit）。
// 这里给出明确的提示，而不是沉默地什么都不做。

var errAutostartUnsupported = errors.New("开机自启目前只做了 Windows 版")

func enableAutostart(bool) error { return errAutostartUnsupported }

func disableAutostart() error { return errAutostartUnsupported }

func autostartStatus() (bool, string) { return false, "" }

func describeAutostart() string { return "未开启（这个平台还没做）" }

func openSelfDir() error { return errAutostartUnsupported }
