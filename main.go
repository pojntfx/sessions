package main

import (
	"fmt"
	"os"
	"time"

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
	onActivate := func(gio.Application) {
		if w != nil {
			w.Present()

			return
		}

		aboutDialog := adw.NewAboutDialogFromAppdata(resources.ResourceMetainfoPath, resources.AppVersion)
		aboutDialog.SetDevelopers(resources.AppDevelopers)
		aboutDialog.SetArtists(resources.AppArtists)
		aboutDialog.SetCopyright(resources.AppCopyright)

		b := gtk.NewBuilderFromResource(resources.ResourceWindowUIPath)

		var (
			win    adw.ApplicationWindow
			label  gtk.Label
			action gtk.Button
			plus   gtk.Button
			minus  gtk.Button
		)
		b.GetObject("main_window").Cast(&win)
		b.GetObject("analog_time_label").Cast(&label)
		b.GetObject("action_button").Cast(&action)
		b.GetObject("plus_button").Cast(&plus)
		b.GetObject("minus_button").Cast(&minus)

		w = &win

		var (
			alarmClockElapsedFile = gtk.NewMediaFileForResource(resources.ResourceAlarmClockElapsedPath)
		)
		startAlarmPlayback := func() {
			alarmClockElapsedFile.Seek(0)
			alarmClockElapsedFile.Play()
		}

		stopAlarmPlayback := func() {
			alarmClockElapsedFile.SetPlaying(false)
			alarmClockElapsedFile.Seek(0)
		}

		var (
			totalSec = 300
			running  = false
		)
		updateButtons := func() {
			if running {
				action.SetIconName("media-playback-stop-symbolic")
				action.SetLabel("Stop") // TODO: Use i18n
				action.RemoveCssClass("suggested-action")
				action.AddCssClass("destructive-action")
			} else {
				action.SetIconName("media-playback-start-symbolic")
				action.SetLabel("Start Timer")
				action.RemoveCssClass("destructive-action")
				action.AddCssClass("suggested-action")
			}

			plus.SetSensitive(totalSec < 3600)
			minus.SetSensitive(totalSec > 30)
		}

		var (
			remain = time.Duration(0)
		)
		updateDial := func() {
			var m, s int
			if running {
				m, s = int(remain.Minutes()), int(remain.Seconds())%60
			} else {
				m, s = totalSec/60, totalSec%60
			}

			label.SetText(fmt.Sprintf("%02d:%02d", m, s))

			// TODO: Add dialer with GSK
		}

		var (
			timer = uint(0)
		)
		createSessionFinishedHandler := func() glib.SourceFunc {
			return func(uintptr) bool {
				if !running {
					return false
				}

				remain -= time.Second
				updateDial()

				if remain <= 0 {
					running = false
					if timer > 0 {
						glib.SourceRemove(timer)
						timer = 0
					}

					updateButtons()
					updateDial()

					startAlarmPlayback()

					n := gio.NewNotification("Session finished") // TODO: Use i18n
					n.SetPriority(gio.GNotificationPriorityUrgentValue)
					n.SetDefaultAction("app.stopAlarmPlayback")
					n.AddButton("Stop alarm", "app.stopAlarmPlayback") // TODO: Use i18n

					a.SendNotification("session-finished", n)

					return false
				}

				return true
			}
		}

		startTimer := func() {
			stopAlarmPlayback()

			running = true
			remain = time.Duration(totalSec) * time.Second

			updateButtons()
			updateDial()

			cb := createSessionFinishedHandler()
			timer = glib.TimeoutAdd(1000, &cb, 0)
		}

		stopTimer := func() {
			running = false
			if timer > 0 {
				glib.SourceRemove(timer)
				timer = 0
			}

			updateButtons()
			updateDial()
		}

		resumeTimer := func() {
			if remain > 0 {
				cb := createSessionFinishedHandler()
				timer = glib.TimeoutAdd(1000, &cb, 0)
			}
		}

		var (
			dragging = false
			paused   = false
		)
		handleDialing := func(x, y float64) {
			if running && !dragging {
				paused = true
				if timer > 0 {
					glib.SourceRemove(timer)
					timer = 0
				}
			}

			// TODO: Add dialer with GSK

			updateDial()
		}

		// TODO: Add dialer with GSK

		drag := gtk.NewGestureDrag()
		onDragBegin := func(_ gtk.GestureDrag, x float64, y float64) {
			dragging = true
			handleDialing(x, y)
		}
		drag.ConnectDragBegin(&onDragBegin)
		onDragUpdate := func(drag gtk.GestureDrag, dx float64, dy float64) {
			if dragging {
				var x, y float64
				drag.GetStartPoint(x, y) // TODO: Fix the bindings here (these should be pointers I think)
				handleDialing(x+dx, y+dy)
			}
		}
		drag.ConnectDragUpdate(&onDragUpdate)
		onDragEnd := func(_ gtk.GestureDrag, dx float64, dy float64) {
			dragging = false

			if paused {
				paused = false
				resumeTimer()
			} else if !running && totalSec > 0 {
				startTimer()
			}
		}
		drag.ConnectDragEnd(&onDragEnd)

		click := gtk.NewGestureClick()
		onPress := func(_ gtk.GestureClick, _ int, x float64, y float64) {
			handleDialing(x, y)
		}
		click.ConnectPressed(&onPress)

		// TODO: Add dialer with GSK

		toggleTimerAction := gio.NewSimpleAction("toggleTimer", nil)
		onToggleTimer := func(gio.SimpleAction, uintptr) {
			if running {
				stopTimer()
			} else if totalSec > 0 {
				startTimer()
			}
		}
		toggleTimerAction.ConnectActivate(&onToggleTimer)
		a.AddAction(toggleTimerAction)

		addTimeAction := gio.NewSimpleAction("addTime", nil)
		onAddTime := func(gio.SimpleAction, uintptr) {
			if totalSec < 3600 {
				totalSec += 30
				if running {
					remain = time.Duration(totalSec) * time.Second
				}

				updateDial()
				updateButtons()
			}
		}
		addTimeAction.ConnectActivate(&onAddTime)
		a.AddAction(addTimeAction)

		removeTimeAction := gio.NewSimpleAction("removeTime", nil)
		onRemoveTime := func(gio.SimpleAction, uintptr) {
			if totalSec > 30 {
				totalSec -= 30
				if running {
					remain = time.Duration(totalSec) * time.Second
				}

				updateDial()
				updateButtons()
			}
		}
		removeTimeAction.ConnectActivate(&onRemoveTime)
		a.AddAction(removeTimeAction)

		openAboutAction := gio.NewSimpleAction("openAbout", nil)
		onOpenAbout := func(gio.SimpleAction, uintptr) {
			aboutDialog.Present(&w.Widget)
		}
		openAboutAction.ConnectActivate(&onOpenAbout)
		a.AddAction(openAboutAction)

		stopAlarmPlaybackAction := gio.NewSimpleAction("stopAlarmPlayback", nil)
		onStopAlarmPlaybackAction := func(gio.SimpleAction, uintptr) {
			stopAlarmPlayback()

			a.Activate()
		}
		stopAlarmPlaybackAction.ConnectActivate(&onStopAlarmPlaybackAction)
		a.AddAction(stopAlarmPlaybackAction)

		updateButtons()
		updateDial()

		a.AddWindow(&w.Window)
		w.Present()
	}
	a.ConnectActivate(&onActivate)

	if code := a.Run(len(os.Args), os.Args); code > 0 {
		os.Exit(code)
	}
}
