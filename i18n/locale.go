package i18n

import "os"

// ===== Internal =====

func environmentLocales() []string {
	locales := make([]string, 0, 3)
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := os.Getenv(name)
		if value == "" {
			continue
		}
		locales = append(locales, value)
	}
	return locales
}
