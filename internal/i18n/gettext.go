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

func Local(input string) string {
	return gettext(input)
}
