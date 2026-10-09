package i18n

import (
	"syscall"
	"unsafe"
)

// ===== Constants =====

const localeNameLength = 85

var userLocaleName = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

// ===== Internal =====

func systemLocales() []string {
	buffer := make([]uint16, localeNameLength)
	length, _, _ := userLocaleName.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return environmentLocales()
	}
	return append([]string{syscall.UTF16ToString(buffer)}, environmentLocales()...)
}
