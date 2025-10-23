package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gdk"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gobject"
	"github.com/jwijenbergh/puregotk/v4/graphene"
	"github.com/jwijenbergh/puregotk/v4/gsk"
	"github.com/jwijenbergh/puregotk/v4/gtk"
	"github.com/pojntfx/sessions/assets/resources"
)

//go:generate sh -c "if [ -z \"$FLATPAK_ID\" ]; then go tool github.com/dennwc/flatpak-go-mod --json .; fi"

/*
#cgo pkg-config: glib-2.0
#include <locale.h>
#include <glib/gi18n.h>
*/
import "C"

const (
	dataKeyGoInstance = "go_instance"

	gettextPackage = "sessions"
	localeDir      = "/usr/share/locale"
)

var (
	gTypeDialWidget          gobject.Type
	gTypeSessionsMainWindow  gobject.Type
	gTypeSessionsApplication gobject.Type
)

func Gettext(key string) string {
	return C.GoString(C.dgettext(C.CString(gettextPackage), C.CString(key)))
}

type dialWidget struct {
	gtk.Widget
	totalSec int
	running  bool
	remain   time.Duration
}

type sessionsMainWindow struct {
	adw.ApplicationWindow
	dialWidget            *dialWidget
	label                 *gtk.Label
	actionButton          *gtk.Button
	plusButton            *gtk.Button
	minusButton           *gtk.Button
	alarmClockElapsedFile *gtk.MediaFile
	app                   *adw.Application
	totalSec              int
	running               bool
	remain                time.Duration
	timer                 uint
	dragging              bool
	paused                bool
}

type sessionsApplication struct {
	adw.Application
	window      *sessionsMainWindow
	aboutDialog *adw.AboutDialog
}

func (w *sessionsMainWindow) startAlarmPlayback() {
	w.alarmClockElapsedFile.Seek(0)
	w.alarmClockElapsedFile.Play()
}

func (w *sessionsMainWindow) stopAlarmPlayback() {
	w.alarmClockElapsedFile.SetPlaying(false)
	w.alarmClockElapsedFile.Seek(0)
}

func (w *sessionsMainWindow) updateButtons() {
	if w.running {
		w.actionButton.SetIconName("media-playback-stop-symbolic")
		w.actionButton.SetLabel(Gettext("Stop"))
		w.actionButton.RemoveCssClass("suggested-action")
		w.actionButton.AddCssClass("destructive-action")
	} else {
		w.actionButton.SetIconName("media-playback-start-symbolic")
		w.actionButton.SetLabel(Gettext("Start Timer"))
		w.actionButton.RemoveCssClass("destructive-action")
		w.actionButton.AddCssClass("suggested-action")
	}

	w.plusButton.SetSensitive(w.totalSec < 3600)
	w.minusButton.SetSensitive(w.totalSec > 30)
}

func (w *sessionsMainWindow) updateDial() {
	var m, s int
	if w.running {
		m, s = int(w.remain.Minutes()), int(w.remain.Seconds())%60
	} else {
		m, s = w.totalSec/60, w.totalSec%60
	}

	w.label.SetText(fmt.Sprintf("%02d:%02d", m, s))

	dialW := (*dialWidget)(unsafe.Pointer(w.dialWidget.GetData(dataKeyGoInstance)))
	if dialW != nil {
		dialW.totalSec = w.totalSec
		dialW.running = w.running
		dialW.remain = w.remain
		w.dialWidget.Widget.QueueDraw()
	}
}

func (w *sessionsMainWindow) createSessionFinishedHandler() glib.SourceFunc {
	return func(uintptr) bool {
		if !w.running {
			return false
		}

		w.remain -= time.Second
		w.updateDial()

		if w.remain <= 0 {
			w.running = false
			if w.timer > 0 {
				glib.SourceRemove(w.timer)
				w.timer = 0
			}

			w.updateButtons()
			w.updateDial()

			w.startAlarmPlayback()

			n := gio.NewNotification(Gettext("Session finished"))
			n.SetPriority(gio.GNotificationPriorityUrgentValue)
			n.SetDefaultAction("app.stopAlarmPlayback")
			n.AddButton(Gettext("Stop alarm"), "app.stopAlarmPlayback")

			w.app.SendNotification("session-finished", n)

			return false
		}

		return true
	}
}

func (w *sessionsMainWindow) startTimer() {
	w.stopAlarmPlayback()

	w.running = true
	w.remain = time.Duration(w.totalSec) * time.Second

	w.updateButtons()
	w.updateDial()

	cb := w.createSessionFinishedHandler()
	w.timer = glib.TimeoutAdd(1000, &cb, 0)
}

func (w *sessionsMainWindow) stopTimer() {
	w.running = false
	if w.timer > 0 {
		glib.SourceRemove(w.timer)
		w.timer = 0
	}

	w.updateButtons()
	w.updateDial()
}

func (w *sessionsMainWindow) resumeTimer() {
	if w.remain > 0 {
		cb := w.createSessionFinishedHandler()
		w.timer = glib.TimeoutAdd(1000, &cb, 0)
	}
}

func (w *sessionsMainWindow) handleDialing(x, y float64) {
	if w.running && !w.dragging {
		w.paused = true
		if w.timer > 0 {
			glib.SourceRemove(w.timer)
			w.timer = 0
		}
	}

	width, height := float64(w.dialWidget.Widget.GetWidth()), float64(w.dialWidget.Widget.GetHeight())
	cx, cy := width/2, height/2
	dx, dy := x-cx, y-cy

	if math.Sqrt(dx*dx+dy*dy) < 15 {
		return
	}

	a := math.Atan2(dy, dx) + math.Pi/2
	if a < 0 {
		a += 2 * math.Pi
	}

	intervals := int((a / (2 * math.Pi)) * 120)
	if intervals == 0 {
		intervals = 120
	}

	w.totalSec = intervals * 30

	if w.paused {
		w.remain = time.Duration(w.totalSec) * time.Second
	}

	w.updateDial()
	w.updateButtons()
}

func (w *sessionsMainWindow) setupDialGestures() {
	drag := gtk.NewGestureDrag()
	onDragBegin := func(_ gtk.GestureDrag, x float64, y float64) {
		w.dragging = true
		w.handleDialing(x, y)
	}
	drag.ConnectDragBegin(&onDragBegin)
	onDragUpdate := func(drag gtk.GestureDrag, dx float64, dy float64) {
		if w.dragging {
			var x, y float64
			drag.GetStartPoint(x, y)
			w.handleDialing(x+dx, y+dy)
		}
	}
	drag.ConnectDragUpdate(&onDragUpdate)
	onDragEnd := func(_ gtk.GestureDrag, dx float64, dy float64) {
		w.dragging = false

		if w.paused {
			w.paused = false
			w.resumeTimer()
		} else if !w.running && w.totalSec > 0 {
			w.startTimer()
		}
	}
	drag.ConnectDragEnd(&onDragEnd)

	click := gtk.NewGestureClick()
	onPress := func(_ gtk.GestureClick, _ int, x float64, y float64) {
		w.handleDialing(x, y)
	}
	click.ConnectPressed(&onPress)

	w.dialWidget.Widget.AddController(&drag.EventController)
	w.dialWidget.Widget.AddController(&click.Gesture.EventController)
}

func (w *sessionsMainWindow) toggleTimer() {
	if w.running {
		w.stopTimer()
	} else if w.totalSec > 0 {
		w.startTimer()
	}
}

func (w *sessionsMainWindow) addTime() {
	if w.totalSec < 3600 {
		w.totalSec += 30
		if w.running {
			w.remain = time.Duration(w.totalSec) * time.Second
		}

		w.updateDial()
		w.updateButtons()
	}
}

func (w *sessionsMainWindow) removeTime() {
	if w.totalSec > 30 {
		w.totalSec -= 30
		if w.running {
			w.remain = time.Duration(w.totalSec) * time.Second
		}

		w.updateDial()
		w.updateButtons()
	}
}

func init() {
	if C.bindtextdomain(C.CString(gettextPackage), C.CString(localeDir)) == nil {
		panic("failed to bind text domain")
	}

	if C.bind_textdomain_codeset(C.CString(gettextPackage), C.CString("UTF-8")) == nil {
		panic("failed to set text domain codeset")
	}

	if C.textdomain(C.CString(gettextPackage)) == nil {
		panic("failed to set text domain")
	}

	var classInit gobject.ClassInitFunc = func(tc *gobject.TypeClass, u uintptr) {
		objClass := (*gobject.ObjectClass)(unsafe.Pointer(tc))

		objClass.OverrideConstructed(func(o *gobject.Object) {
			parentObjClass := (*gobject.ObjectClass)(unsafe.Pointer(tc.PeekParent()))
			parentObjClass.GetConstructed()(o)

			var parent gtk.Widget
			o.Cast(&parent)

			w := &dialWidget{
				Widget:   parent,
				totalSec: 300,
				running:  false,
				remain:   0,
			}

			var pinner runtime.Pinner
			pinner.Pin(w)

			var cleanupCallback glib.DestroyNotify = func(data uintptr) {
				pinner.Unpin()
			}
			o.SetDataFull(dataKeyGoInstance, uintptr(unsafe.Pointer(w)), &cleanupCallback)
		})

		widgetClass := (*gtk.WidgetClass)(unsafe.Pointer(tc))

		widgetClass.OverrideSnapshot(func(widget *gtk.Widget, snapshot *gtk.Snapshot) {
			dialW := (*dialWidget)(unsafe.Pointer(widget.GetData(dataKeyGoInstance)))
			if dialW == nil {
				return
			}

			w := float64(widget.GetWidth())
			h := float64(widget.GetHeight())
			cx := w / 2
			cy := h / 2
			r := math.Min(cx, cy) - 15

			styleContext := widget.GetStyleContext()
			var accent, errColor gdk.RGBA
			styleContext.LookupColor("accent_bg_color", &accent)
			styleContext.LookupColor("error_bg_color", &errColor)

			grayColor := gdk.RGBA{
				Red:   0.7,
				Green: 0.7,
				Blue:  0.7,
				Alpha: 1.0,
			}

			fullCircleBuilder := gsk.NewPathBuilder()
			centerPoint := graphene.Point{X: float32(cx), Y: float32(cy)}
			fullCircleBuilder.AddCircle(&centerPoint, float32(r))
			fullCirclePath := fullCircleBuilder.ToPath()
			fullCircleStroke := gsk.NewStroke(10.0)
			snapshot.AppendStroke(fullCirclePath, fullCircleStroke, &grayColor)

			if dialW.totalSec > 0 {
				progress := float64(dialW.totalSec) / 3600.0
				end := -math.Pi/2 + 2*math.Pi*progress
				var angle float64
				var lineColor gdk.RGBA
				var fillR, fillG, fillB, fillA float32

				if dialW.running && dialW.remain > 0 {
					ratio := dialW.remain.Seconds() / float64(dialW.totalSec)
					angle = -math.Pi/2 + 2*math.Pi*progress*ratio
					lineColor = errColor
					fillR, fillG, fillB, fillA = errColor.Red, errColor.Green, errColor.Blue, 0.3
				} else {
					angle = end
					lineColor = accent
					fillR, fillG, fillB, fillA = 0.6, 0.6, 0.6, 0.2
				}

				fillColor := gdk.RGBA{
					Red:   fillR,
					Green: fillG,
					Blue:  fillB,
					Alpha: fillA,
				}

				startX := float32(cx)
				startY := float32(cy - r)
				endX := float32(cx + r*math.Sin(angle+math.Pi/2))
				endY := float32(cy - r*math.Cos(angle+math.Pi/2))

				arcBuilder := gsk.NewPathBuilder()
				arcBuilder.MoveTo(float32(cx), float32(cy))
				arcBuilder.LineTo(startX, startY)
				arcBuilder.SvgArcTo(float32(r), float32(r), 0, angle+math.Pi/2 > math.Pi, true, endX, endY)
				arcBuilder.LineTo(float32(cx), float32(cy))
				arcPath := arcBuilder.ToPath()
				snapshot.AppendFill(arcPath, gsk.FillRuleWindingValue, &fillColor)

				lineStrokeColor := gdk.RGBA{
					Red:   lineColor.Red,
					Green: lineColor.Green,
					Blue:  lineColor.Blue,
					Alpha: 1.0,
				}

				arcLineBuilder := gsk.NewPathBuilder()
				arcLineBuilder.MoveTo(startX, startY)
				arcLineBuilder.SvgArcTo(float32(r), float32(r), 0, angle+math.Pi/2 > math.Pi, true, endX, endY)
				arcLinePath := arcLineBuilder.ToPath()
				arcStroke := gsk.NewStroke(10.0)
				arcStroke.SetLineCap(gsk.LineCapRoundValue)
				snapshot.AppendStroke(arcLinePath, arcStroke, &lineStrokeColor)

				handleX := float32(cx + r*math.Cos(angle))
				handleY := float32(cy + r*math.Sin(angle))

				handleBuilder := gsk.NewPathBuilder()
				handlePoint := graphene.Point{X: handleX, Y: handleY}
				handleBuilder.AddCircle(&handlePoint, 8)
				handlePath := handleBuilder.ToPath()
				snapshot.AppendFill(handlePath, gsk.FillRuleWindingValue, &lineColor)
			}
		})
	}

	var instanceInit gobject.InstanceInitFunc = func(ti *gobject.TypeInstance, tc *gobject.TypeClass) {}

	var parentQuery gobject.TypeQuery
	gobject.NewTypeQuery(gtk.WidgetGLibType(), &parentQuery)

	gTypeDialWidget = gobject.TypeRegisterStaticSimple(
		parentQuery.Type,
		"DialWidget",
		parentQuery.ClassSize,
		&classInit,
		parentQuery.InstanceSize+uint(unsafe.Sizeof(dialWidget{}))+uint(unsafe.Sizeof(&dialWidget{})),
		&instanceInit,
		0,
	)

	var windowClassInit gobject.ClassInitFunc = func(tc *gobject.TypeClass, u uintptr) {
		typeClass := (*gtk.WidgetClass)(unsafe.Pointer(tc))
		typeClass.SetTemplateFromResource(resources.ResourceWindowUIPath)

		typeClass.BindTemplateChildFull("analog_time_label", false, 0)
		typeClass.BindTemplateChildFull("action_button", false, 0)
		typeClass.BindTemplateChildFull("plus_button", false, 0)
		typeClass.BindTemplateChildFull("minus_button", false, 0)
		typeClass.BindTemplateChildFull("dial_area", false, 0)

		objClass := (*gobject.ObjectClass)(unsafe.Pointer(tc))

		objClass.OverrideConstructed(func(o *gobject.Object) {
			parentObjClass := (*gobject.ObjectClass)(unsafe.Pointer(tc.PeekParent()))
			parentObjClass.GetConstructed()(o)

			var parent adw.ApplicationWindow
			o.Cast(&parent)

			parent.InitTemplate()

			var (
				label        gtk.Label
				actionButton gtk.Button
				plusButton   gtk.Button
				minusButton  gtk.Button
				dialArea     gtk.Box
			)
			parent.Widget.GetTemplateChild(
				gTypeSessionsMainWindow,
				"analog_time_label",
			).Cast(&label)
			parent.Widget.GetTemplateChild(
				gTypeSessionsMainWindow,
				"action_button",
			).Cast(&actionButton)
			parent.Widget.GetTemplateChild(
				gTypeSessionsMainWindow,
				"plus_button",
			).Cast(&plusButton)
			parent.Widget.GetTemplateChild(
				gTypeSessionsMainWindow,
				"minus_button",
			).Cast(&minusButton)
			parent.Widget.GetTemplateChild(
				gTypeSessionsMainWindow,
				"dial_area",
			).Cast(&dialArea)

			dialObj := gobject.NewObject(gTypeDialWidget, "css-name")
			var dial dialWidget
			dialObj.Cast(&dial)
			dial.Widget.SetHexpand(true)
			dial.Widget.SetVexpand(true)
			dialArea.Append(&dial.Widget)

			w := &sessionsMainWindow{
				ApplicationWindow:     parent,
				dialWidget:            &dial,
				label:                 &label,
				actionButton:          &actionButton,
				plusButton:            &plusButton,
				minusButton:           &minusButton,
				alarmClockElapsedFile: gtk.NewMediaFileForResource(resources.ResourceAlarmClockElapsedPath),
				totalSec:              300,
				running:               false,
				remain:                0,
				timer:                 0,
				dragging:              false,
				paused:                false,
			}

			var pinner runtime.Pinner
			pinner.Pin(w)

			var cleanupCallback glib.DestroyNotify = func(data uintptr) {
				pinner.Unpin()
			}
			o.SetDataFull(dataKeyGoInstance, uintptr(unsafe.Pointer(w)), &cleanupCallback)

			w.setupDialGestures()
		})
	}

	var windowInstanceInit gobject.InstanceInitFunc = func(ti *gobject.TypeInstance, tc *gobject.TypeClass) {}

	var windowParentQuery gobject.TypeQuery
	gobject.NewTypeQuery(adw.ApplicationWindowGLibType(), &windowParentQuery)

	gTypeSessionsMainWindow = gobject.TypeRegisterStaticSimple(
		windowParentQuery.Type,
		"SessionsMainWindow",
		windowParentQuery.ClassSize,
		&windowClassInit,
		windowParentQuery.InstanceSize+uint(unsafe.Sizeof(sessionsMainWindow{}))+uint(unsafe.Sizeof(&sessionsMainWindow{})),
		&windowInstanceInit,
		0,
	)

	var appClassInit gobject.ClassInitFunc = func(tc *gobject.TypeClass, u uintptr) {
		objClass := (*gobject.ObjectClass)(unsafe.Pointer(tc))

		objClass.OverrideConstructed(func(o *gobject.Object) {
			parentObjClass := (*gobject.ObjectClass)(unsafe.Pointer(tc.PeekParent()))

			parentObjClass.GetConstructed()(o)

			var parent adw.Application
			o.Cast(&parent)

			app := &sessionsApplication{
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
			sessionsApp := (*sessionsApplication)(unsafe.Pointer(a.GetData(dataKeyGoInstance)))

			if sessionsApp.window != nil {
				sessionsApp.window.ApplicationWindow.Present()
				return
			}

			sessionsApp.aboutDialog = adw.NewAboutDialogFromAppdata(resources.ResourceMetainfoPath, resources.AppVersion)
			sessionsApp.aboutDialog.SetDevelopers(resources.AppDevelopers)
			sessionsApp.aboutDialog.SetArtists(resources.AppArtists)
			sessionsApp.aboutDialog.SetCopyright(resources.AppCopyright)

			var app gtk.Application
			a.Cast(&app)

			obj := gobject.NewObject(gTypeSessionsMainWindow,
				"application", app,
			)

			sessionsApp.window = (*sessionsMainWindow)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
			sessionsApp.window.app = &sessionsApp.Application

			sessionsApp.window.updateButtons()
			sessionsApp.window.updateDial()

			toggleTimerAction := gio.NewSimpleAction("toggleTimer", nil)
			onToggleTimer := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.toggleTimer()
			}
			toggleTimerAction.ConnectActivate(&onToggleTimer)
			sessionsApp.Application.AddAction(toggleTimerAction)

			addTimeAction := gio.NewSimpleAction("addTime", nil)
			onAddTime := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.addTime()
			}
			addTimeAction.ConnectActivate(&onAddTime)
			sessionsApp.Application.AddAction(addTimeAction)

			removeTimeAction := gio.NewSimpleAction("removeTime", nil)
			onRemoveTime := func(gio.SimpleAction, uintptr) {
				sessionsApp.window.removeTime()
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
				sessionsApp.window.stopAlarmPlayback()
				sessionsApp.Application.Activate()
			}
			stopAlarmPlaybackAction.ConnectActivate(&onStopAlarmPlaybackAction)
			sessionsApp.Application.AddAction(stopAlarmPlaybackAction)

			sessionsApp.Application.AddWindow(&sessionsApp.window.ApplicationWindow.Window)
			sessionsApp.window.ApplicationWindow.Present()
		})
	}

	var appInstanceInit gobject.InstanceInitFunc = func(ti *gobject.TypeInstance, tc *gobject.TypeClass) {}

	var appParentQuery gobject.TypeQuery
	gobject.NewTypeQuery(adw.ApplicationGLibType(), &appParentQuery)

	gTypeSessionsApplication = gobject.TypeRegisterStaticSimple(
		appParentQuery.Type,
		"SessionsApplication",
		appParentQuery.ClassSize,
		&appClassInit,
		appParentQuery.InstanceSize+uint(unsafe.Sizeof(sessionsApplication{}))+uint(unsafe.Sizeof(&sessionsApplication{}))+uint(unsafe.Sizeof(&adw.ApplicationWindow{}))+uint(unsafe.Sizeof(&dialWidget{})),
		&appInstanceInit,
		0,
	)
}

func main() {
	resource, err := gio.NewResourceFromData(glib.NewBytes(resources.ResourceContents, uint(len(resources.ResourceContents))))
	if err != nil {
		panic(err)
	}
	gio.ResourcesRegister(resource)

	obj := gobject.NewObject(gTypeSessionsApplication,
		"application_id", resources.AppID,
		"flags", gio.GApplicationDefaultFlagsValue,
	)

	var app sessionsApplication
	obj.Cast(&app)

	os.Exit(app.Run(len(os.Args), os.Args))
}
