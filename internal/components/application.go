package components

import (
	"runtime"
	"unsafe"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gobject"
	"github.com/jwijenbergh/puregotk/v4/gtk"
	. "github.com/pojntfx/go-gettext/pkg/i18n"
	"github.com/pojntfx/sessions/assets/resources"
)

var (
	gTypeApplication gobject.Type
)

type Application struct {
	adw.Application

	window      *MainWindow
	aboutDialog *adw.AboutDialog
	settings    *gio.Settings
}

func NewApplication(settings *gio.Settings, FirstPropertyNameVar string, varArgs ...interface{}) Application {
	obj := gobject.NewObject(gTypeApplication, FirstPropertyNameVar, varArgs...)

	var v Application
	obj.Cast(&v)

	app := (*Application)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
	app.settings = settings

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

			obj := NewMainWindow(&sessionsApp.Application, "application", app)

			sessionsApp.window = (*MainWindow)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
			sessionsApp.window.settings = sessionsApp.settings
			sessionsApp.window.LoadLastPosition()

			sessionsApp.window.UpdateButtons()
			sessionsApp.window.UpdateDial()

			toggleTimerAction := gio.NewSimpleAction("toggleTimer", nil)
			onToggleTimer := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.ToggleTimer()
			}
			toggleTimerAction.ConnectActivate(&onToggleTimer)
			sessionsApp.Application.AddAction(toggleTimerAction)

			addTimeAction := gio.NewSimpleAction("addTime", nil)
			onAddTime := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.AddTime()
			}
			addTimeAction.ConnectActivate(&onAddTime)
			sessionsApp.Application.AddAction(addTimeAction)

			removeTimeAction := gio.NewSimpleAction("removeTime", nil)
			onRemoveTime := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.RemoveTime()
			}
			removeTimeAction.ConnectActivate(&onRemoveTime)
			sessionsApp.Application.AddAction(removeTimeAction)

			openAboutAction := gio.NewSimpleAction("openAbout", nil)
			onOpenAbout := func(gio.SimpleAction, uintptr) {
				sessionsApp.aboutDialog.Present(&sessionsApp.window.ApplicationWindow.Widget)
			}
			openAboutAction.ConnectActivate(&onOpenAbout)
			sessionsApp.Application.AddAction(openAboutAction)

			stopAlarmPlaybackAction := gio.NewSimpleAction("stopAlarmPlayback", nil)
			onStopAlarmPlaybackAction := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.StopAlarmPlayback()
				sessionsApp.Application.Activate()
			}
			stopAlarmPlaybackAction.ConnectActivate(&onStopAlarmPlaybackAction)
			sessionsApp.Application.AddAction(stopAlarmPlaybackAction)

			closeWindowAction := gio.NewSimpleAction("closeWindow", nil)
			onCloseWindow := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.ApplicationWindow.Close()
			}
			closeWindowAction.ConnectActivate(&onCloseWindow)
			sessionsApp.Application.AddAction(closeWindowAction)

			quitAction := gio.NewSimpleAction("quit", nil)
			onQuit := func(gio.SimpleAction, uintptr) {
				sessionsApp.Application.Quit()
			}
			quitAction.ConnectActivate(&onQuit)
			sessionsApp.Application.AddAction(quitAction)

			sessionsApp.Application.SetAccelsForAction("app.shortcuts", []string{`<Primary>question`})
			sessionsApp.Application.SetAccelsForAction("app.closeWindow", []string{`<Primary>w`})
			sessionsApp.Application.SetAccelsForAction("app.quit", []string{`<Primary>q`})
			sessionsApp.Application.SetAccelsForAction("app.toggleTimer", []string{`<Primary>space`})
			sessionsApp.Application.SetAccelsForAction("app.addTime", []string{`<Primary>plus`, `<Primary>equal`})
			sessionsApp.Application.SetAccelsForAction("app.removeTime", []string{`<Primary>minus`})

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
		appParentQuery.InstanceSize+uint(unsafe.Sizeof(Application{}))+uint(unsafe.Sizeof(&Application{}))+uint(unsafe.Sizeof(&adw.ApplicationWindow{}))+uint(unsafe.Sizeof(&Dial{})),
		&appInstanceInit,
		0,
	)
}
