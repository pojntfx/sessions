package i18n

/*
#cgo pkg-config: glib-2.0
#include <locale.h>
#include <glib/gi18n.h>
*/
import "C"
import (
	"errors"
	"unsafe"
)

func InitI18n(domain, dir string) error {
	cDomain := C.CString(domain)
	defer C.free(unsafe.Pointer(cDomain))

	cDir := C.CString(dir)
	defer C.free(unsafe.Pointer(cDir))

	if C.bindtextdomain(cDomain, cDir) == nil {
		return errors.New("failed to bind text domain")
	}

	cUTF8 := C.CString("UTF-8")
	defer C.free(unsafe.Pointer(cUTF8))

	if C.bind_textdomain_codeset(cDomain, cUTF8) == nil {
		return errors.New("failed to set text domain codeset")
	}

	if C.textdomain(cDomain) == nil {
		return errors.New("failed to set text domain")
	}

	return nil
}

func Local(input string) string {
	cstr := C.CString(input)
	defer C.free(unsafe.Pointer(cstr))

	return C.GoString(C.gettext(cstr))
}
