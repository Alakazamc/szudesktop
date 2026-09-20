//go:build !windows

package ui

import (
	"context"
	"errors"
)

func recognizeCalendar(context.Context, []byte) (string, error) {
	return "", errors.New("calendar image OCR requires Windows")
}
