//go:build !windows

package i18n

import (
	"os/exec"
	"runtime"
	"strings"
)

// ===== Internal =====

func systemLocales() []string {
	if runtime.GOOS != "darwin" {
		return environmentLocales()
	}
	output, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return environmentLocales()
	}
	return append([]string{strings.TrimSpace(string(output))}, environmentLocales()...)
}
