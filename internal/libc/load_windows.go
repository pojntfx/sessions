// Imported with changes from https://github.com/ebitengine/purego/blob/main/examples/libc/main_unix.go

package libc

import "syscall"

func OpenLibrary(name string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(name)

	return uintptr(handle), err
}
