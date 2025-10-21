package main

import (
	"os"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gtk"
	"github.com/pojntfx/sessions/assets/resources"
)

//go:generate sh -c "if [ -z \"$FLATPAK_ID\" ]; then go tool github.com/dennwc/flatpak-go-mod --json .; fi"

func main() {
	resource, err := gio.NewResourceFromData(glib.NewBytes(resources.ResourceContents, uint(len(resources.ResourceContents))))
	if err != nil {
		panic(err)
	}
	gio.ResourcesRegister(resource)

	a := adw.NewApplication(resources.AppID, gio.GApplicationDefaultFlagsValue)

	var w *adw.ApplicationWindow
	cb := func(gio.Application) {
		if w != nil {
			w.Present()

			return
		}

		aboutDialog := adw.NewAboutDialogFromAppdata(resources.ResourceMetainfoPath, resources.AppVersion)
		aboutDialog.SetDevelopers(resources.AppDevelopers)
		aboutDialog.SetArtists(resources.AppArtists)
		aboutDialog.SetCopyright(resources.AppCopyright)

		b := gtk.NewBuilderFromResource(resources.ResourceWindowUIPath)

		var win adw.ApplicationWindow
		b.GetObject("main_window").Cast(&win)
		w = &win

		openAboutAction := gio.NewSimpleAction("openAbout", nil)
		cb := func(gio.SimpleAction, uintptr) {
			aboutDialog.Present(&w.Widget)
		}
		openAboutAction.ConnectActivate(&cb)
		a.AddAction(openAboutAction)

		a.AddWindow(&w.Window)
		w.Present()
	}
	a.ConnectActivate(&cb)

	if code := a.Run(len(os.Args), os.Args); code > 0 {
		os.Exit(code)
	}
}
