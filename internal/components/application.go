package components

import (
	"context"
	"log/slog"
	"runtime"
	"unsafe"

	"codeberg.org/puregotk/puregotk/v4/adw"
	"codeberg.org/puregotk/puregotk/v4/gio"
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gobject"
	"codeberg.org/puregotk/puregotk/v4/gtk"
	. "github.com/pojntfx/go-gettext/pkg/i18n"
	"github.com/pojntfx/sessions/assets/resources"
)

var (
	gTypeApplication gobject.Type
)

type Application struct {
	adw.Application

	ctx      context.Context
	settings *gio.Settings
	log      *slog.Logger

	window      *MainWindow
	aboutDialog *adw.AboutDialog
}

func NewApplication(ctx context.Context, settings *gio.Settings, log *slog.Logger, FirstPropertyNameVar string, varArgs ...interface{}) Application {
	obj := gobject.NewObject(gTypeApplication, FirstPropertyNameVar, varArgs...)

	var v Application
	obj.Cast(&v)

	app := (*Application)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))

	app.ctx = ctx
	app.settings = settings
	app.log = log

	return v
}

func init() {
	var appClassInit gobject.ClassInitFunc = func(tc *gobject.TypeClass, u uintptr) {
		objClass := (*gobject.ObjectClass)(unsafe.Pointer(tc))

		objClass.OverrideConstructed(func(o *gobject.Object) {
			parentObjClass := (*gobject.ObjectClass)(unsafe.Pointer(tc.PeekParent()))

			parentObjClass.GetConstructed()(o)

			var parent adw.Application
			o.Cast(&parent)

			app := &Application{
				Application: parent,
			}

			var pinner runtime.Pinner
			pinner.Pin(app)

			var cleanupCallback glib.DestroyNotify = func(data uintptr) {
				pinner.Unpin()
			}
			o.SetDataFull(dataKeyGoInstance, uintptr(unsafe.Pointer(app)), &cleanupCallback)
		})

		applicationClass := (*gio.ApplicationClass)(unsafe.Pointer(tc))

		applicationClass.OverrideActivate(func(a *gio.Application) {
			sessionsApp := (*Application)(unsafe.Pointer(a.GetData(dataKeyGoInstance)))

			if sessionsApp.window != nil {
				sessionsApp.window.ApplicationWindow.Present()
				return
			}

			sessionsApp.aboutDialog = adw.NewAboutDialogFromAppdata(resources.ResourceMetainfoPath, resources.AppVersion)
			sessionsApp.aboutDialog.SetDevelopers(resources.AppDevelopers)
			sessionsApp.aboutDialog.SetArtists(resources.AppArtists)
			sessionsApp.aboutDialog.SetCopyright(resources.AppCopyright)
			// TRANSLATORS: Replace "translator-credits" with your name/username, and optionally an email or URL.
			sessionsApp.aboutDialog.SetTranslatorCredits(L("translator-credits"))

			var app gtk.Application
			a.Cast(&app)

			obj := NewMainWindow(sessionsApp.ctx, &sessionsApp.Application, sessionsApp.log, sessionsApp.settings, "application", app)

			sessionsApp.window = (*MainWindow)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))

			openAboutAction := gio.NewSimpleAction("openAbout", nil)
			onOpenAbout := func(gio.SimpleAction, uintptr) {
				sessionsApp.aboutDialog.Present(&sessionsApp.window.ApplicationWindow.Widget)
			}
			openAboutAction.ConnectActivate(&onOpenAbout)
			sessionsApp.Application.AddAction(openAboutAction)

			quitAction := gio.NewSimpleAction("quit", nil)
			onQuit := func(gio.SimpleAction, uintptr) {
				sessionsApp.Application.Quit()
			}
			quitAction.ConnectActivate(&onQuit)
			sessionsApp.Application.AddAction(quitAction)

			sessionsApp.Application.SetAccelsForAction("app.shortcuts", []string{`<Primary>question`})
			sessionsApp.Application.SetAccelsForAction("app.quit", []string{`<Primary>q`})

			sessionsApp.Application.AddWindow(&sessionsApp.window.ApplicationWindow.Window)
			sessionsApp.window.ApplicationWindow.Present()
		})
	}

	var appInstanceInit gobject.InstanceInitFunc = func(ti *gobject.TypeInstance, tc *gobject.TypeClass) {}

	var appParentQuery gobject.TypeQuery
	gobject.NewTypeQuery(adw.ApplicationGLibType(), &appParentQuery)

	gTypeApplication = gobject.TypeRegisterStaticSimple(
		appParentQuery.Type,
		"SessionsApplication",
		appParentQuery.ClassSize,
		&appClassInit,
		appParentQuery.InstanceSize,
		&appInstanceInit,
		0,
	)
}
