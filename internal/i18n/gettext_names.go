// Imported with changes from https://github.com/ebitengine/purego/blob/main/examples/libc/main_unix.go

package i18n

import (
	"errors"
	"path/filepath"
	"runtime"
)

func getGettextLibraryNames() ([]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join("/usr", "local", "lib", "libintl.dylib"),    // Homebrew amd64
			filepath.Join("/opt", "homebrew", "lib", "libintl.dylib"), // Homebrew arm64
			filepath.Join("/opt", "local", "lib", "libintl.dylib"),    // MacPorts
		}, nil
	case "linux":
		return []string{
			"libc.so.6",    // GNU libc
			"libintl.so.8", // musl libc
		}, nil
	case "windows":
		return []string{
			"libintl-8.dll", // Relative to running binary
		}, nil
	default:
		return []string{}, errors.New("this GOOS is not supported")
	}
}
