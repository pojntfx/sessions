// Inspired by https://github.com/diamondburned/gotk4/blob/d44ab4b5b24e200c90c6a9ffb6632ada7e166a79/pkg/core/glib/glib.go#L50-L72

package i18n

import (
	"errors"

	"github.com/jwijenbergh/purego"
)

var (
	bindtextdomain        func(domainname string, dirname string) string
	bindTextdomainCodeset func(domainname string, codeset string) string
	textdomain            func(domainname string) string
	gettext               func(msgid string) string
)

// InitI18n initializes the i18n subsystem. It runs the following C code:
//
//	setlocale(LC_ALL, "");
//	bindtextdomain(domain, dir);
//	bind_textdomain_codeset(domain, "UTF-8");
//	textdomain(domain);
func InitI18n(domain, dir string) error {
	gettextLibNames, err := getGettextLibraryNames()
	if err != nil {
		return errors.Join(errors.New("could get gettext library names"), err)
	}

	var libc uintptr
	for _, gettextLibName := range gettextLibNames {
		libc, err = openLibrary(gettextLibName)
		if err != nil {
			return errors.Join(errors.New("could not open library"), err)
		} else {
			break
		}
	}

	purego.RegisterLibFunc(&bindtextdomain, libc, "bindtextdomain")
	purego.RegisterLibFunc(&bindTextdomainCodeset, libc, "bind_textdomain_codeset")
	purego.RegisterLibFunc(&textdomain, libc, "textdomain")
	purego.RegisterLibFunc(&gettext, libc, "gettext")

	if bindtextdomain(domain, dir) == "" {
		return errors.New("failed to bind text domain")
	}

	if bindTextdomainCodeset(domain, "UTF-8") == "" {
		return errors.New("failed to set text domain codeset")
	}

	if textdomain(domain) == "" {
		return errors.New("failed to set text domain")
	}

	return nil
}

// Local localizes a string using gettext.
func Local(input string) string {
	return gettext(input)
}
