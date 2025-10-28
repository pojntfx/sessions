// Imported with changes from https://github.com/ebitengine/purego/blob/main/examples/libc/main_unix.go

//go:build darwin || freebsd || linux || netbsd

package libc

import "github.com/jwijenbergh/purego"

func OpenLibrary(name string) (uintptr, error) {
	return purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}
