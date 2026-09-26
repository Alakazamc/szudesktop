//go:build !windows

package ui

import (
	"context"
	"errors"
)

func recognizeCalendar(context.Context, []byte) (string, error) {
	return "", errors.New("本机暂不支持校历 OCR（仅 Windows）")
}
