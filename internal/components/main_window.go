package components

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
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
	dragging              bool
	paused                bool
	held                  bool

	ctx context.Context
	s   *state.StateMachine
}

func NewMainWindow(ctx context.Context, app *adw.Application, log *slog.Logger, settings *gio.Settings, FirstPropertyNameVar string, varArgs ...interface{}) MainWindow {
	obj := gobject.NewObject(gTypeMainWindow, FirstPropertyNameVar, varArgs...)

	var v MainWindow
	obj.Cast(&v)

	window := (*MainWindow)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
	window.app = app
	window.log = log
	window.settings = settings
	window.ctx = ctx

	dial := NewDial(app, "css-name")
	dial.Widget.SetHexpand(true)
	dial.Widget.SetVexpand(true)
	window.dialArea.Append(&dial.Widget)
	window.dialWidget = &dial

	var remainingToLabel gobject.BindingTransformFunc = func(_ uintptr, from *gobject.Value, to *gobject.Value, _ uintptr) bool {
		remainingTime := int(from.GetInt())
		to.SetString(fmt.Sprintf("%02d:%02d", remainingTime/60, remainingTime%60))
		return true
	}
	dial.Widget.Object.BindPropertyFull(
		"remaining-time",
		&window.label.Widget.Object,
		"label",
		gobject.GBindingSyncCreateValue,
		&remainingToLabel,
		nil,
		0,
		nil,
	)

	onDialDragBegin := func() {
		if window.alarming {
			return
		}

		if window.running {
			window.paused = true
		}
		window.dragging = true
	}
	dial.ConnectDragBegin(&onDialDragBegin)

	onDialDragEnd := func() {
		window.dragging = false

		if window.alarming {
			return
		}

		remainingTime := dial.GetRemainingTime()
		window.totalSec = remainingTime
		if window.paused {
			window.remain = time.Duration(remainingTime) * time.Second
			window.paused = false
		} else if !window.running && remainingTime > 0 {
			window.startTimer()
		}
		window.saveLastPosition()
	}
	dial.ConnectDragEnd(&onDialDragEnd)

	window.loadLastPosition()
	window.registerActions()
	window.updateButtons()
	window.updateDial()

	return v
}

func (w *MainWindow) loadLastPosition() {
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
			OnAfterStartingTimer: func(ctx context.Context) error {
				w.stopAlarmPlayback()

				w.running = true
				w.remain = time.Duration(w.totalSec) * time.Second

				w.updateButtons()
				w.updateDial()

				w.holdApp()

				return nil
			},

			OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
				w.totalSec = int(initialRemainingTime.Seconds())
				if w.running {
					w.remain = initialRemainingTime
				}

				w.updateDial()
				w.updateButtons()
				w.saveLastPosition()

				return nil
			},

			OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error {
				w.remain = currentRemainingTime

				w.updateDial()

				return nil
			},

			OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
			OnAfterStoppingTimer: func(ctx context.Context) error {
				w.running = false

				w.updateButtons()
				w.updateDial()

				w.stopAlarmPlayback()

				w.releaseApp()

				return nil
			},

			OnStartAlarm: func(ctx context.Context) error {
				w.running = false

				w.startAlarmPlayback()

				w.updateButtons()
				w.updateDial()

				n := gio.NewNotification(L("Session Finished"))
				n.SetBody(L("Time to take a break"))
				n.SetPriority(gio.GNotificationPriorityHighValue)
				// We need to attach to `app`, not `win` since it's possible that no window
				// is focused when the notification is activated
				n.SetDefaultAction("app.stopAlarmPlayback")

				w.app.SendNotification(notificationIdVar, n)

				return nil
			},

			OnStopAlarm: func(ctx context.Context) error { return nil },

			OnPermittedTriggersChange: func(ctx context.Context, permittedTriggers []state.Trigger) error {
				if slices.Contains(permittedTriggers, state.TriggerMinusTimer) {
					fn := glib.SourceFunc(func(_ uintptr) bool {
						w.minusButton.SetSensitive(true)

						return false
					})
					glib.IdleAdd(&fn, 0)
				} else {
					fn := glib.SourceFunc(func(_ uintptr) bool {
						w.minusButton.SetSensitive(false)

						return false
					})
					glib.IdleAdd(&fn, 0)
				}

				if slices.Contains(permittedTriggers, state.TriggerPlusTimer) {
					fn := glib.SourceFunc(func(_ uintptr) bool {
						w.plusButton.SetSensitive(true)

						return false
					})
					glib.IdleAdd(&fn, 0)
				} else {
					fn := glib.SourceFunc(func(_ uintptr) bool {
						w.plusButton.SetSensitive(false)

						return false
					})
					glib.IdleAdd(&fn, 0)
				}

				return nil
			},
		},
	)
	w.s.FlushPermittedTriggers(w.ctx)
}

func (w *MainWindow) saveLastPosition() {
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

func (w *MainWindow) startAlarmPlayback() {
	w.alarming = true
	w.alarmClockElapsedFile.Seek(0)
	w.alarmClockElapsedFile.Play()

	w.label.Announce(L("Session Finished"), gtk.AccessibleAnnouncementPriorityHighValue)

	w.startFlash()
	w.releaseApp()
}

func (w *MainWindow) stopAlarmPlayback() {
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

	w.updateButtons()
	w.updateDial()

	w.releaseApp()
}

func (w *MainWindow) startFlash() {
	w.label.AddCssClass("dial__display--alarming")
}

func (w *MainWindow) stopFlash() {
	w.label.RemoveCssClass("dial__display--alarming")
}

func (w *MainWindow) updateButtons() {
	if w.alarming || w.running {
		w.actionButton.SetIconName("media-playback-stop-symbolic")
		w.actionButton.SetLabel(L("_Stop"))
		w.actionButton.RemoveCssClass("suggested-action")
		w.actionButton.AddCssClass("destructive-action")
	} else {
		w.actionButton.SetIconName("media-playback-start-symbolic")
		w.actionButton.SetLabel(L("_Start Timer"))
		w.actionButton.RemoveCssClass("destructive-action")
		w.actionButton.AddCssClass("suggested-action")
	}
}

func (w *MainWindow) updateDial() {
	if w.alarming {
		w.dialWidget.SetCountingDown(true)
		w.dialWidget.SetRemainingTime(0)
	} else if w.running {
		w.dialWidget.SetCountingDown(true)
		w.dialWidget.SetRemainingTime(int(w.remain.Seconds()))
	} else {
		w.dialWidget.SetCountingDown(false)
		w.dialWidget.SetRemainingTime(w.totalSec)
	}
}

func (w *MainWindow) startTimer() {
	if err := w.s.StartTimer(w.ctx); err != nil {
		w.log.Error("Could not start timer", "err", err)
	}
}

func (w *MainWindow) stopTimer() {
	if err := w.s.StopTimer(w.ctx); err != nil {
		w.log.Error("Could not stop timer", "err", err)
	}
}

func (w *MainWindow) toggleTimer() {
	if w.alarming {
		w.stopAlarming()
		w.stopAlarmPlayback()
	} else if w.running {
		w.stopTimer()
	} else if w.totalSec > 0 {
		w.startTimer()
	}
}

func (w *MainWindow) addTime() {
	if w.alarming {
		return
	}

	if err := w.s.PlusTimer(w.ctx); err != nil {
		w.log.Error("Could not add time", "err", err)
	}
}

func (w *MainWindow) removeTime() {
	if w.alarming {
		return
	}

	if err := w.s.MinusTimer(w.ctx); err != nil {
		w.log.Error("Could not remove time", "err", err)
	}
}

func (w *MainWindow) registerActions() {
	toggleTimerAction := gio.NewSimpleAction("toggleTimer", nil)
	onToggleTimer := func(gio.SimpleAction, uintptr) {
		w.toggleTimer()
	}
	toggleTimerAction.ConnectActivate(&onToggleTimer)
	w.AddAction(toggleTimerAction)

	addTimeAction := gio.NewSimpleAction("addTime", nil)
	onAddTime := func(gio.SimpleAction, uintptr) {
		w.addTime()
	}
	addTimeAction.ConnectActivate(&onAddTime)
	w.AddAction(addTimeAction)

	removeTimeAction := gio.NewSimpleAction("removeTime", nil)
	onRemoveTime := func(gio.SimpleAction, uintptr) {
		w.removeTime()
	}
	removeTimeAction.ConnectActivate(&onRemoveTime)
	w.AddAction(removeTimeAction)

	closeWindowAction := gio.NewSimpleAction("closeWindow", nil)
	onCloseWindow := func(gio.SimpleAction, uintptr) {
		w.Close()
	}
	closeWindowAction.ConnectActivate(&onCloseWindow)
	w.AddAction(closeWindowAction)

	stopAlarmPlaybackAction := gio.NewSimpleAction("stopAlarmPlayback", nil)
	onStopAlarmPlaybackAction := func(gio.SimpleAction, uintptr) {
		w.stopAlarmPlayback()
		w.app.Activate()
	}
	stopAlarmPlaybackAction.ConnectActivate(&onStopAlarmPlaybackAction)
	w.app.AddAction(stopAlarmPlaybackAction)

	w.app.SetAccelsForAction("win.closeWindow", []string{`<Primary>w`})
	w.app.SetAccelsForAction("win.toggleTimer", []string{`<Primary>space`})
	w.app.SetAccelsForAction("win.addTime", []string{`<Primary>plus`, `<Primary>equal`})
	w.app.SetAccelsForAction("win.removeTime", []string{`<Primary>minus`})
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
