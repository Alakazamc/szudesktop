// Package version 是全项目版本号的唯一来源。
//
// 为什么用 embed 而不是构建时注入 -ldflags：版本号原本散落在两个 main.go、
// 页面和打包脚本里，改一处忘一处就会出现「关于页写的版本和 exe 实际版本不一致」。
// 现在只有同目录的 VERSION 一个文件，go build 直接把它编进二进制，
// 任何构建方式（本地、CI、用户自己 go build）拿到的都是同一个值，
// 不依赖调用方记得传 ldflags。
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Current 是编译进二进制的版本号，例如 beta0.6.1。
var Current = strings.TrimSpace(raw)
