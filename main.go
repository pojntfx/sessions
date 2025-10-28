package main

import (
	"os"

	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/pojntfx/go-gettext/pkg/i18n"
	"github.com/pojntfx/sessions/assets/resources"
	"github.com/pojntfx/sessions/internal/components"
)

//go:generate sh -c "if [ -z \"$FLATPAK_ID\" ]; then go tool github.com/dennwc/flatpak-go-mod --json .; fi"

const (
	gettextPackage = "sessions"
	localeDir      = "/usr/share/locale"
)

func init() {
	if err := i18n.InitI18n(gettextPackage, localeDir); err != nil {
		panic(err)
	}
}

func main() {
	resource, err := gio.NewResourceFromData(glib.NewBytes(resources.ResourceContents, uint(len(resources.ResourceContents))))
	if err != nil {
		panic(err)
	}
	gio.ResourcesRegister(resource)

	app := components.NewApplication(
		"application_id", resources.AppID,
		"flags", gio.GApplicationDefaultFlagsValue,
	)

	os.Exit(app.Run(len(os.Args), os.Args))
}
