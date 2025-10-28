// Imported with changes from https://github.com/ebitengine/purego/blob/main/examples/libc/main_unix.go

package libc

import (
	"errors"
	"path/filepath"
	"runtime"
)

var (
	ErrGOOSNotSupported = errors.New("This GOOS is not supported")
)

func GetLibCName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join("/usr", "lib", "libSystem.B.dylib"), nil
	case "freebsd":
		return "libc.so.7", nil
	case "linux":
		return "libc.so.6", nil
	case "netbsd":
		return "libc.so", nil
	case "windows":
		return "ucrtbase.dll", nil
	default:
		return "", ErrGOOSNotSupported
	}
}
