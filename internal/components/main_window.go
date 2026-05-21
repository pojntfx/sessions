package components

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"time"
	"unsafe"

	"codeberg.org/puregotk/puregotk/v4/adw"
	"codeberg.org/puregotk/puregotk/v4/gio"
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gobject"
	"codeberg.org/puregotk/puregotk/v4/gtk"
	. "github.com/pojntfx/go-gettext/pkg/i18n"
	"github.com/pojntfx/sessions/assets/resources"
	"github.com/pojntfx/sessions/pkg/state"
	"github.com/rymdport/portal/background"
)

var (
	gTypeMainWindow gobject.Type
)

const (
	minDialValue = state.MinInitialRemainingTime
	maxDialValue = state.MaxInitialRemainingTime

	notificationIdVar = "session-finished"
)

type MainWindow struct {
	adw.ApplicationWindow

	dialWidget            *Dial
	dialArea              gtk.Box
	label                 *gtk.Label
	actionButton          *gtk.Button
	plusButton            *gtk.Button
	minusButton           *gtk.Button
	alarmClockElapsedFile *gtk.MediaFile
	app                   *adw.Application
	settings              *gio.Settings
	log                   *slog.Logger
	totalSec              int
	running               bool
	alarming              bool
	remain                time.Duration
	timer                 uint32
	dragging              bool
	paused                bool
	held                  bool

	ctx context.Context
	s   *state.StateMachine
}

func NewMainWindow(ctx context.Context, app *adw.Application, log *slog.Logger, FirstPropertyNameVar string, varArgs ...interface{}) MainWindow {
	obj := gobject.NewObject(gTypeMainWindow, FirstPropertyNameVar, varArgs...)

	var v MainWindow
	obj.Cast(&v)

	window := (*MainWindow)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
	window.app = app
	window.log = log

	dial := NewDial(app, "css-name")
	dial.Widget.SetHexpand(true)
	dial.Widget.SetVexpand(true)
	window.dialArea.Append(&dial.Widget)
	window.dialWidget = &dial
	window.setupDialGestures()

	window.ctx = ctx

	return v
}

func (w *MainWindow) LoadLastPosition() {
	lastPosition := w.settings.GetInt64(resources.SchemaLastPositionKey)
	if lastPosition >= int64(minDialValue.Seconds()) && lastPosition <= int64(maxDialValue.Seconds()) {
		w.totalSec = int(lastPosition)
	}

	w.s = state.NewStateMachine(
		w.ctx,
		time.Second*time.Duration(w.totalSec),
		w.log,
		&state.Hooks{
			OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
			OnAfterStartingTimer:  func(ctx context.Context) error { return nil },

			OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
				w.totalSec = int(initialRemainingTime.Seconds())
				if w.running {
					w.remain = initialRemainingTime
				}

				w.UpdateDial()
				w.UpdateButtons()
				w.SaveLastPosition()

				return nil
			},
			OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error { return nil },

			OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
			OnAfterStoppingTimer:  func(ctx context.Context) error { return nil },

			OnStartAlarm: func(ctx context.Context) error { return nil },

			OnStopAlarm: func(ctx context.Context) error { return nil },

			OnPermittedTriggersChange: func(ctx context.Context, permittedTriggers []state.Trigger) error { return nil },
		},
	)
}

func (w *MainWindow) SaveLastPosition() {
	w.settings.SetInt64(resources.SchemaLastPositionKey, int64(w.totalSec))
}

func (w *MainWindow) holdApp() {
	if !w.held {
		w.held = true

		res, err := background.RequestBackground("", &background.RequestOptions{
			// TRANSLATORS: Reason given when requesting permission to run in the background from the system.
			Reason: L("Running the Timer in the Background"),
		})
		if err != nil {
			w.log.Error("Could not request permission to run in background via background portal", "err", err)

			return
		}

		if !res.Background { // Permission to run the background was denied
			w.held = false

			return
		}

		if err := background.SetStatus(background.StatusOptions{
			// TRANSLATORS: Message shown in the background apps list next to the app while the app is running in the background.
			Message: L("Timer Running"),
		},
		); err != nil {
			w.log.Error("Could not set app status via background portal", "err", err)

			return
		}

		w.app.Hold()
		w.SetHideOnClose(true)
	}
}

func (w *MainWindow) releaseApp() {
	if w.held {
		w.held = false

		if err := background.SetStatus(background.StatusOptions{
			Message: "",
		},
		); err != nil {
			w.log.Error("Could not clear app status via background portal", "err", err)

			return
		}

		w.SetHideOnClose(false)
		w.app.Release()
	}
}

func (w *MainWindow) StartAlarmPlayback() {
	w.alarming = true
	w.alarmClockElapsedFile.Seek(0)
	w.alarmClockElapsedFile.Play()

	w.label.Announce(L("Session Finished"), gtk.AccessibleAnnouncementPriorityHighValue)

	w.startFlash()
	w.releaseApp()
}

func (w *MainWindow) StopAlarmPlayback() {
	w.alarmClockElapsedFile.SetPlaying(false)
	w.alarmClockElapsedFile.Seek(0)

	w.app.WithdrawNotification(notificationIdVar)

	if w.alarming {
		w.stopAlarming()
	}
}

func (w *MainWindow) stopAlarming() {
	w.alarming = false
	w.running = false
	w.stopFlash()

	w.UpdateButtons()
	w.UpdateDial()

	w.releaseApp()
}

func (w *MainWindow) startFlash() {
	w.label.AddCssClass("dial__display--alarming")
}

func (w *MainWindow) stopFlash() {
	w.label.RemoveCssClass("dial__display--alarming")
}

func (w *MainWindow) UpdateButtons() {
	if w.alarming {
		w.actionButton.SetIconName("media-playback-stop-symbolic")
		w.actionButton.SetLabel(L("_Stop"))
		w.actionButton.RemoveCssClass("suggested-action")
		w.actionButton.AddCssClass("destructive-action")

		w.plusButton.SetSensitive(false)
		w.minusButton.SetSensitive(false)
	} else if w.running {
		w.actionButton.SetIconName("media-playback-stop-symbolic")
		w.actionButton.SetLabel(L("_Stop"))
		w.actionButton.RemoveCssClass("suggested-action")
		w.actionButton.AddCssClass("destructive-action")

		w.plusButton.SetSensitive(w.totalSec < int(maxDialValue.Seconds()))
		w.minusButton.SetSensitive(w.totalSec > int(minDialValue.Seconds()))
	} else {
		w.actionButton.SetIconName("media-playback-start-symbolic")
		w.actionButton.SetLabel(L("_Start Timer"))
		w.actionButton.RemoveCssClass("destructive-action")
		w.actionButton.AddCssClass("suggested-action")

		w.plusButton.SetSensitive(w.totalSec < int(maxDialValue.Seconds()))
		w.minusButton.SetSensitive(w.totalSec > int(minDialValue.Seconds()))
	}
}

func (w *MainWindow) UpdateDial() {
	var m, s int
	if w.alarming {
		m, s = 0, 0
		w.dialWidget.SetTimer(w.totalSec, true, 0)
	} else if w.running {
		m, s = int(w.remain.Minutes()), int(w.remain.Seconds())%60
		w.dialWidget.SetTimer(w.totalSec, w.running, w.remain)
	} else {
		m, s = w.totalSec/60, w.totalSec%60
		w.dialWidget.SetTimer(w.totalSec, w.running, w.remain)
	}

	w.label.SetText(fmt.Sprintf("%02d:%02d", m, s))
}

func (w *MainWindow) createSessionFinishedHandler() glib.SourceFunc {
	return func(uintptr) bool {
		if !w.running {
			return false
		}

		w.remain -= time.Second
		w.UpdateDial()

		if w.remain <= 0 {
			w.running = false
			if w.timer > 0 {
				glib.SourceRemove(w.timer)
				w.timer = 0
			}

			w.StartAlarmPlayback()

			w.UpdateButtons()
			w.UpdateDial()

			n := gio.NewNotification(L("Session Finished"))
			n.SetBody(L("Time to take a break"))
			n.SetPriority(gio.GNotificationPriorityHighValue)
			n.SetDefaultAction("app.stopAlarmPlayback")

			w.app.SendNotification(notificationIdVar, n)

			return false
		}

		return true
	}
}

func (w *MainWindow) StartTimer() {
	w.StopAlarmPlayback()

	w.running = true
	w.remain = time.Duration(w.totalSec) * time.Second

	w.UpdateButtons()
	w.UpdateDial()

	cb := w.createSessionFinishedHandler()
	w.timer = glib.TimeoutAdd(1000, &cb, 0)

	w.holdApp()
}

func (w *MainWindow) StopTimer() {
	w.running = false
	if w.timer > 0 {
		glib.SourceRemove(w.timer)
		w.timer = 0
	}

	w.UpdateButtons()
	w.UpdateDial()

	w.StopAlarmPlayback()

	w.releaseApp()
}

func (w *MainWindow) resumeTimer() {
	if w.remain > 0 {
		cb := w.createSessionFinishedHandler()
		w.timer = glib.TimeoutAdd(1000, &cb, 0)
	}
}

func (w *MainWindow) handleDialing(x, y float64) {
	if w.alarming {
		return
	}

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

	w.totalSec = intervals * int(minDialValue.Seconds())

	if w.paused {
		w.remain = time.Duration(w.totalSec) * time.Second
	}

	w.UpdateDial()
	w.SaveLastPosition()
}

func (w *MainWindow) setupDialGestures() {
	drag := gtk.NewGestureDrag()
	onDragBegin := func(_ gtk.GestureDrag, x float64, y float64) {
		if w.alarming {
			return
		}

		w.dragging = true
		w.handleDialing(x, y)
	}
	drag.ConnectDragBegin(&onDragBegin)
	onDragUpdate := func(drag gtk.GestureDrag, dx float64, dy float64) {
		if w.dragging {
			var x, y float64
			drag.GetStartPoint(&x, &y)
			w.handleDialing(x+dx, y+dy)
		}
	}
	drag.ConnectDragUpdate(&onDragUpdate)
	onDragEnd := func(_ gtk.GestureDrag, dx float64, dy float64) {
		w.dragging = false

		if w.alarming {
			return
		}

		if w.paused {
			w.paused = false
			w.resumeTimer()
		} else if !w.running && w.totalSec > 0 {
			w.StartTimer()
		}
	}
	drag.ConnectDragEnd(&onDragEnd)

	click := gtk.NewGestureClick()
	onPress := func(_ gtk.GestureClick, _ int32, x float64, y float64) {
		if w.alarming {
			return
		}

		w.handleDialing(x, y)
	}
	click.ConnectPressed(&onPress)

	w.dialWidget.Widget.AddController(&drag.EventController)
	w.dialWidget.Widget.AddController(&click.Gesture.EventController)
}

func (w *MainWindow) ToggleTimer() {
	if w.alarming {
		w.stopAlarming()
		w.StopAlarmPlayback()
	} else if w.running {
		w.StopTimer()
	} else if w.totalSec > 0 {
		w.StartTimer()
	}
}

func (w *MainWindow) AddTime() {
	if w.alarming {
		return
	}

	if err := w.s.PlusTimer(w.ctx); err != nil {
		w.log.Error("Could not add time", "err", err)
	}
}

func (w *MainWindow) RemoveTime() {
	if w.alarming {
		return
	}

	if err := w.s.MinusTimer(w.ctx); err != nil {
		w.log.Error("Could not remove time", "err", err)
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

			w := &MainWindow{
				ApplicationWindow: parent,

				dialArea:              dialArea,
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
		})
	}

	var windowInstanceInit gobject.InstanceInitFunc = func(ti *gobject.TypeInstance, tc *gobject.TypeClass) {}

	var windowParentQuery gobject.TypeQuery
	gobject.NewTypeQuery(adw.ApplicationWindowGLibType(), &windowParentQuery)

	gTypeMainWindow = gobject.TypeRegisterStaticSimple(
		windowParentQuery.Type,
		"SessionsMainWindow",
		windowParentQuery.ClassSize,
		&windowClassInit,
		windowParentQuery.InstanceSize,
		&windowInstanceInit,
		0,
	)
}
