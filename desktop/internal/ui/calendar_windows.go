//go:build windows

package ui

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

//go:embed calendar-ocr.ps1
var calendarOCR string

func recognizeCalendar(ctx context.Context, image []byte) (string, error) {
	f, err := os.CreateTemp("", "szudesktop-calendar-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(image)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	words := utf16.Encode([]rune(calendarOCR))
	encoded := make([]byte, len(words)*2)
	for i, w := range words {
		binary.LittleEndian.PutUint16(encoded[i*2:], w)
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Env = append(os.Environ(), "SZU_CALENDAR_IMAGE="+f.Name())
	out, err := cmd.Output()
	return strings.TrimPrefix(string(out), "\ufeff"), err
}
