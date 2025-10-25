package components

import (
	"fmt"
	"math"
	"runtime"
	"time"
	"unsafe"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gio"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gobject"
	"github.com/jwijenbergh/puregotk/v4/gtk"
	"github.com/pojntfx/sessions/assets/resources"
	"github.com/pojntfx/sessions/internal/i18n"
)

var (
	gTypeMainWindow gobject.Type
)

type MainWindow struct {
	adw.ApplicationWindow

	dialWidget            *Dial
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

func NewMainWindow(FirstPropertyNameVar string, varArgs ...interface{}) MainWindow {
	obj := gobject.NewObject(gTypeMainWindow, FirstPropertyNameVar, varArgs...)

	var v MainWindow
	obj.Cast(&v)

	return v
}

func (w *MainWindow) startAlarmPlayback() {
	w.alarmClockElapsedFile.Seek(0)
	w.alarmClockElapsedFile.Play()
}

func (w *MainWindow) stopAlarmPlayback() {
	w.alarmClockElapsedFile.SetPlaying(false)
	w.alarmClockElapsedFile.Seek(0)
}

func (w *MainWindow) updateButtons() {
	if w.running {
		w.actionButton.SetIconName("media-playback-stop-symbolic")
		w.actionButton.SetLabel(i18n.Local("Stop"))
		w.actionButton.RemoveCssClass("suggested-action")
		w.actionButton.AddCssClass("destructive-action")
	} else {
		w.actionButton.SetIconName("media-playback-start-symbolic")
		w.actionButton.SetLabel(i18n.Local("Start Timer"))
		w.actionButton.RemoveCssClass("destructive-action")
		w.actionButton.AddCssClass("suggested-action")
	}

	w.plusButton.SetSensitive(w.totalSec < 3600)
	w.minusButton.SetSensitive(w.totalSec > 30)
}

func (w *MainWindow) updateDial() {
	var m, s int
	if w.running {
		m, s = int(w.remain.Minutes()), int(w.remain.Seconds())%60
	} else {
		m, s = w.totalSec/60, w.totalSec%60
	}

	w.label.SetText(fmt.Sprintf("%02d:%02d", m, s))

	dialW := (*Dial)(unsafe.Pointer(w.dialWidget.GetData(dataKeyGoInstance)))
	if dialW != nil {
		dialW.totalSec = w.totalSec
		dialW.running = w.running
		dialW.remain = w.remain
		w.dialWidget.Widget.QueueDraw()
	}
}

func (w *MainWindow) createSessionFinishedHandler() glib.SourceFunc {
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

			n := gio.NewNotification(i18n.Local("Session finished"))
			n.SetPriority(gio.GNotificationPriorityUrgentValue)
			n.SetDefaultAction("app.stopAlarmPlayback")
			n.AddButton(i18n.Local("Stop alarm"), "app.stopAlarmPlayback")

			w.app.SendNotification("session-finished", n)

			return false
		}

		return true
	}
}

func (w *MainWindow) startTimer() {
	w.stopAlarmPlayback()

	w.running = true
	w.remain = time.Duration(w.totalSec) * time.Second

	w.updateButtons()
	w.updateDial()

	cb := w.createSessionFinishedHandler()
	w.timer = glib.TimeoutAdd(1000, &cb, 0)
}

func (w *MainWindow) stopTimer() {
	w.running = false
	if w.timer > 0 {
		glib.SourceRemove(w.timer)
		w.timer = 0
	}

	w.updateButtons()
	w.updateDial()
}

func (w *MainWindow) resumeTimer() {
	if w.remain > 0 {
		cb := w.createSessionFinishedHandler()
		w.timer = glib.TimeoutAdd(1000, &cb, 0)
	}
}

func (w *MainWindow) handleDialing(x, y float64) {
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

func (w *MainWindow) setupDialGestures() {
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

func (w *MainWindow) toggleTimer() {
	if w.running {
		w.stopTimer()
	} else if w.totalSec > 0 {
		w.startTimer()
	}
}

func (w *MainWindow) addTime() {
	if w.totalSec < 3600 {
		w.totalSec += 30
		if w.running {
			w.remain = time.Duration(w.totalSec) * time.Second
		}

		w.updateDial()
		w.updateButtons()
	}
}

func (w *MainWindow) removeTime() {
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
				gTypeMainWindow,
				"analog_time_label",
			).Cast(&label)
			parent.Widget.GetTemplateChild(
				gTypeMainWindow,
				"action_button",
			).Cast(&actionButton)
			parent.Widget.GetTemplateChild(
				gTypeMainWindow,
				"plus_button",
			).Cast(&plusButton)
			parent.Widget.GetTemplateChild(
				gTypeMainWindow,
				"minus_button",
			).Cast(&minusButton)
			parent.Widget.GetTemplateChild(
				gTypeMainWindow,
				"dial_area",
			).Cast(&dialArea)

			dial := NewDial("css-name")

			dial.Widget.SetHexpand(true)
			dial.Widget.SetVexpand(true)
			dialArea.Append(&dial.Widget)

			w := &MainWindow{
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

	gTypeMainWindow = gobject.TypeRegisterStaticSimple(
		windowParentQuery.Type,
		"MainWindow",
		windowParentQuery.ClassSize,
		&windowClassInit,
		windowParentQuery.InstanceSize+uint(unsafe.Sizeof(MainWindow{}))+uint(unsafe.Sizeof(&MainWindow{})),
		&windowInstanceInit,
		0,
	)
}
