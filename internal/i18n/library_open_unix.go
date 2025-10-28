// Imported with changes from https://github.com/ebitengine/purego/blob/main/examples/libc/main_unix.go

//go:build linux || darwin

package i18n

import "github.com/jwijenbergh/purego"

func openLibrary(name string) (uintptr, error) {
	return purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}
