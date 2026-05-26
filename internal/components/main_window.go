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
	notificationIdVar = "session-finished"
)

type MainWindow struct {
	adw.ApplicationWindow

	ctx      context.Context
	settings *gio.Settings
	log      *slog.Logger

	dialWidget   *Dial
	dialArea     gtk.Box
	label        *gtk.Label
	actionButton *gtk.Button
	plusButton   *gtk.Button
	minusButton  *gtk.Button

	alarmClockElapsedFile *gtk.MediaFile

	app *adw.Application

	s    *state.StateMachine
	held bool
}

func NewMainWindow(ctx context.Context, app *adw.Application, log *slog.Logger, settings *gio.Settings, FirstPropertyNameVar string, varArgs ...interface{}) MainWindow {
	obj := gobject.NewObject(gTypeMainWindow, FirstPropertyNameVar, varArgs...)

	var v MainWindow
	obj.Cast(&v)

	window := (*MainWindow)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
	window.settings = settings
	window.log = log

	window.app = app

	window.ctx = ctx

	dial := NewDial(window.app, "css-name")
	dial.Widget.SetHexpand(true)
	dial.Widget.SetVexpand(true)
	window.dialArea.Append(&dial.Widget)
	window.dialWidget = &dial

	var remainingToLabel gobject.BindingTransformFunc = func(_ uintptr, from *gobject.Value, to *gobject.Value, _ uintptr) bool {
		to.SetString(fmt.Sprintf("%02d:%02d", from.GetInt()/60, from.GetInt()%60))

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

	lastInitialRemainingTime := time.Second * time.Duration(window.settings.GetInt64(resources.SchemaLastPositionKey))
	window.dialWidget.SetRemainingTime(int(lastInitialRemainingTime.Seconds()))

	var (
		toggleTimerAction = gio.NewSimpleAction("toggleTimer", nil)
		addTimeAction     = gio.NewSimpleAction("addTime", nil)
		removeTimeAction  = gio.NewSimpleAction("removeTime", nil)

		canStopTimer,
		canStopAlarming bool
	)
	window.s = state.NewStateMachine(
		window.ctx,
		lastInitialRemainingTime,
		window.log,
		&state.Hooks{
			OnBeforeStartingTimer: func(ctx context.Context) error { return nil },
			OnAfterStartingTimer: func(ctx context.Context) error {
				fn := glib.SourceFunc(func(u uintptr) bool {
					window.dialWidget.SetCountingDown(true)

					if !window.held {
						res, err := background.RequestBackground("", &background.RequestOptions{
							// TRANSLATORS: Reason given when requesting permission to run in the background from the system.
							Reason: L("Running the Timer in the Background"),
						})
						if err != nil {
							window.log.Error("Could not request permission to run in background via background portal", "err", err)

							return false
						}

						if !res.Background { // Permission to run the background was denied
							return false
						}

						if err := background.SetStatus(background.StatusOptions{
							// TRANSLATORS: Message shown in the background apps list next to the app while the app is running in the background.
							Message: L("Timer Running"),
						},
						); err != nil {
							window.log.Error("Could not set app status via background portal", "err", err)

							return false
						}

						window.app.Hold()
						window.SetHideOnClose(true)

						window.held = true
					}

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},

			OnInitialRemainingTimeChange: func(ctx context.Context, initialRemainingTime time.Duration) error {
				lastInitialRemainingTime = initialRemainingTime

				fn := glib.SourceFunc(func(u uintptr) bool {
					window.dialWidget.SetRemainingTime(int(lastInitialRemainingTime.Seconds()))
					window.settings.SetInt64(resources.SchemaLastPositionKey, int64(lastInitialRemainingTime.Seconds()))

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},
			OnCurrentRemainingTimeTick: func(ctx context.Context, currentRemainingTime time.Duration) error {
				fn := glib.SourceFunc(func(u uintptr) bool {
					window.dialWidget.SetRemainingTime(int(currentRemainingTime.Seconds()))

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},

			OnBeforeStoppingTimer: func(ctx context.Context) error { return nil },
			OnAfterStoppingTimer: func(ctx context.Context) error {
				fn := glib.SourceFunc(func(u uintptr) bool {
					window.dialWidget.SetRemainingTime(int(lastInitialRemainingTime.Seconds()))

					window.dialWidget.SetCountingDown(false)

					if window.held {
						if err := background.SetStatus(background.StatusOptions{
							Message: "",
						},
						); err != nil {
							window.log.Error("Could not clear app status via background portal", "err", err)

							return false
						}

						window.SetHideOnClose(false)
						window.app.Release()

						window.held = false
					}

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},

			OnStartAlarm: func(ctx context.Context) error {
				fn := glib.SourceFunc(func(u uintptr) bool {
					window.dialWidget.SetRemainingTime(0)

					window.dialWidget.SetCountingDown(true)

					if !window.held {
						res, err := background.RequestBackground("", &background.RequestOptions{
							// TRANSLATORS: Reason given when requesting permission to run in the background from the system.
							Reason: L("Running the Timer in the Background"),
						})
						if err != nil {
							window.log.Error("Could not request permission to run in background via background portal", "err", err)

							return false
						}

						if !res.Background { // Permission to run the background was denied
							return false
						}

						if err := background.SetStatus(background.StatusOptions{
							// TRANSLATORS: Message shown in the background apps list next to the app while the app is running in the background and the session has finished.
							Message: L("Session Finished"),
						},
						); err != nil {
							window.log.Error("Could not set app status via background portal", "err", err)

							return false
						}

						window.app.Hold()
						window.SetHideOnClose(true)

						window.held = true
					}

					window.dialWidget.SetSensitive(false)

					window.label.AddCssClass("dial__display--alarming")

					n := gio.NewNotification(L("Session Finished"))
					n.SetBody(L("Time to take a break"))
					n.SetPriority(gio.GNotificationPriorityHighValue)
					// We need to attach to `app`, not `win` since it's possible that no window
					// is focused when the notification is activated
					n.SetDefaultAction("app.stopAlarmPlayback")

					window.app.SendNotification(notificationIdVar, n)

					window.alarmClockElapsedFile.Seek(0)
					window.alarmClockElapsedFile.Play()

					window.label.Announce(L("Session Finished"), gtk.AccessibleAnnouncementPriorityHighValue)

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},

			OnStopAlarm: func(ctx context.Context) error {
				fn := glib.SourceFunc(func(u uintptr) bool {
					window.dialWidget.SetRemainingTime(int(lastInitialRemainingTime.Seconds()))

					window.dialWidget.SetCountingDown(false)

					if window.held {
						if err := background.SetStatus(background.StatusOptions{
							Message: "",
						},
						); err != nil {
							window.log.Error("Could not clear app status via background portal", "err", err)

							return false
						}

						window.SetHideOnClose(false)
						window.app.Release()

						window.held = false
					}

					window.dialWidget.SetSensitive(true)

					window.label.RemoveCssClass("dial__display--alarming")

					window.alarmClockElapsedFile.SetPlaying(false)
					window.alarmClockElapsedFile.Seek(0)

					window.app.WithdrawNotification(notificationIdVar)

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},

			OnPermittedTriggersChange: func(ctx context.Context, permittedTriggers []state.Trigger) error {
				fn := glib.SourceFunc(func(u uintptr) bool {
					toggleTimerAction.SetEnabled(
						slices.Contains(permittedTriggers, state.TriggerStartTimer) ||
							slices.Contains(permittedTriggers, state.TriggerStopTimer) ||
							slices.Contains(permittedTriggers, state.TriggerStopAlarming),
					)
					removeTimeAction.SetEnabled(slices.Contains(permittedTriggers, state.TriggerMinusTimer))
					addTimeAction.SetEnabled(slices.Contains(permittedTriggers, state.TriggerPlusTimer))

					if slices.Contains(permittedTriggers, state.TriggerStartTimer) {
						window.actionButton.SetIconName("media-playback-start-symbolic")
						window.actionButton.SetLabel(L("_Start Timer"))
						window.actionButton.RemoveCssClass("destructive-action")
						window.actionButton.AddCssClass("suggested-action")
					}

					canStopTimer, canStopAlarming = slices.Contains(permittedTriggers, state.TriggerStopTimer), slices.Contains(permittedTriggers, state.TriggerStopAlarming)
					if canStopTimer || canStopAlarming {
						window.actionButton.SetIconName("media-playback-stop-symbolic")
						window.actionButton.SetLabel(L("_Stop"))
						window.actionButton.RemoveCssClass("suggested-action")
						window.actionButton.AddCssClass("destructive-action")
					}

					return false
				})
				glib.IdleAdd(&fn, 0)

				return nil
			},
		},
	)
	window.s.FlushPermittedTriggers(window.ctx)

	onDialDragBegin := func() {
		if err := window.s.StartDragging(window.ctx); err != nil {
			window.log.Error("Could not start dragging", "err", err)

			return
		}
	}
	dial.ConnectDragBegin(&onDialDragBegin)

	onDialDragEnd := func() {
		if err := window.s.StopDragging(window.ctx, time.Duration(window.dialWidget.GetRemainingTime())*time.Second); err != nil {
			window.log.Error("Could not stop dragging", "err", err)

			return
		}
	}
	dial.ConnectDragEnd(&onDialDragEnd)

	onToggleTimer := func(gio.SimpleAction, uintptr) {
		if canStopTimer {
			if err := window.s.StopTimer(window.ctx); err != nil {
				window.log.Error("Could not stop timer", "err", err)

				return
			}

			return
		}

		if canStopAlarming {
			if err := window.s.StopAlarming(window.ctx); err != nil {
				window.log.Error("Could not stop alarming", "err", err)

				return
			}

			return
		}

		if err := window.s.StartTimer(window.ctx); err != nil {
			window.log.Error("Could not start timer", "err", err)

			return
		}
	}
	toggleTimerAction.ConnectActivate(&onToggleTimer)
	window.AddAction(toggleTimerAction)

	onAddTime := func(gio.SimpleAction, uintptr) {
		if err := window.s.PlusTimer(window.ctx); err != nil {
			window.log.Error("Could not add time to timer", "err", err)

			return
		}
	}
	addTimeAction.ConnectActivate(&onAddTime)
	window.AddAction(addTimeAction)

	onRemoveTime := func(gio.SimpleAction, uintptr) {
		if err := window.s.MinusTimer(window.ctx); err != nil {
			window.log.Error("Could not remove time from timer", "err", err)

			return
		}
	}
	removeTimeAction.ConnectActivate(&onRemoveTime)
	window.AddAction(removeTimeAction)

	closeWindowAction := gio.NewSimpleAction("closeWindow", nil)
	onCloseWindow := func(gio.SimpleAction, uintptr) {
		window.Close()
	}
	closeWindowAction.ConnectActivate(&onCloseWindow)
	window.AddAction(closeWindowAction)

	stopAlarmPlaybackAction := gio.NewSimpleAction("stopAlarmPlayback", nil)
	onStopAlarmPlaybackAction := func(gio.SimpleAction, uintptr) {
		if err := window.s.StopAlarming(window.ctx); err != nil {
			window.log.Error("Could not stop alarming", "err", err)

			return
		}

		window.app.Activate()
	}
	stopAlarmPlaybackAction.ConnectActivate(&onStopAlarmPlaybackAction)
	window.app.AddAction(stopAlarmPlaybackAction)

	window.app.SetAccelsForAction("win.closeWindow", []string{`<Primary>w`})
	window.app.SetAccelsForAction("win.toggleTimer", []string{`<Primary>space`})
	window.app.SetAccelsForAction("win.addTime", []string{`<Primary>plus`, `<Primary>equal`})
	window.app.SetAccelsForAction("win.removeTime", []string{`<Primary>minus`})

	return v
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

				dialArea:     dialArea,
				label:        &label,
				actionButton: &actionButton,
				plusButton:   &plusButton,
				minusButton:  &minusButton,

				alarmClockElapsedFile: gtk.NewMediaFileForResource(resources.ResourceAlarmClockElapsedPath),
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
