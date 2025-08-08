package main

import (
	"os"
	"path"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	_ "embed"
)

const (
	appID      = "com.pojtinger.felicitas.Sessions"
	appVersion = "0.1.0"
)

//go:generate sh -c "blueprint-compiler batch-compile . . *.blp && glib-compile-resources *.gresource.xml"
//go:embed index.gresource
var ResourceContents []byte

var (
	appPath = path.Join("/com", "pojtinger", "felicitas", "Sessions")

	appDevelopers = []string{"Felicitas Pojtinger"}
	appArtists    = appDevelopers
	appCopyright  = "© 2025 " + strings.Join(appDevelopers, ", ")

	resourceWindowUIPath = path.Join(appPath, "window.ui")
	resourceMetainfoPath = path.Join(appPath, "metainfo.xml")
)

func main() {
	r, err := gio.NewResourceFromData(glib.NewBytesWithGo(ResourceContents))
	if err != nil {
		panic(err)
	}
	gio.ResourcesRegister(r)

	a := adw.NewApplication(appID, gio.ApplicationHandlesOpen)

	a.ConnectActivate(func() {
		aboutDialog := adw.NewAboutDialogFromAppdata(resourceMetainfoPath, appVersion)
		aboutDialog.SetDevelopers(appDevelopers)
		aboutDialog.SetArtists(appArtists)
		aboutDialog.SetCopyright(appCopyright)

		b := gtk.NewBuilderFromResource(resourceWindowUIPath)

		w := b.GetObject("main_window").Cast().(*adw.Window)

		openAboutAction := gio.NewSimpleAction("openAbout", nil)
		openAboutAction.ConnectActivate(func(parameter *glib.Variant) {
			aboutDialog.Present(&w.Window)
		})
		a.AddAction(openAboutAction)

		a.AddWindow(&w.Window)
	})

	if code := a.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}
