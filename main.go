package main

import (
	"errors"
	"log/slog"
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
)

var (
	LocaleDir = "/usr/share/locale"
	SchemaDir = ""
)

func init() {
	if err := i18n.InitI18n(gettextPackage, LocaleDir, slog.Default()); err != nil {
		panic(err)
	}

	resource, err := gio.NewResourceFromData(glib.NewBytes(resources.ResourceContents, uint(len(resources.ResourceContents))))
	if err != nil {
		panic(err)
	}
	gio.ResourcesRegister(resource)
}

func main() {
	var settings gio.Settings
	if SchemaDir == "" {
		settings = *gio.NewSettings(resources.AppID)
	} else {
		source, err := gio.NewSettingsSchemaSourceFromDirectory(SchemaDir, gio.SettingsSchemaSourceGetDefault(), true)
		if err != nil {
			panic(err)
		}

		schema := source.Lookup(resources.AppID, false)
		if schema == nil {
			panic(errors.New("could not find schema"))
		}

		settings = *gio.NewSettingsFull(schema, nil, schema.GetPath())
	}

	app := components.NewApplication(
		&settings,
		slog.Default(),
		"application_id", resources.AppID,
		"flags", gio.GApplicationDefaultFlagsValue,
	)

	os.Exit(int(app.Run(int32(len(os.Args)), os.Args)))
}
