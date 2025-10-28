// Imported with changes from https://github.com/ebitengine/purego/blob/main/examples/libc/main_unix.go

package i18n

import "syscall"

func openLibrary(name string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(name)
	if err != nil {
		return 0, err
	}

	return uintptr(handle), nil
}
