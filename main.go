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

const (
	dataKeyGoInstance = "go_instance"
)

var (
	gTypeDialWidget gobject.Type
)

type dialWidget struct {
	gtk.Widget
	totalSec int
	running  bool
	remain   time.Duration
}

func init() {
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
}

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
			win      adw.ApplicationWindow
			label    gtk.Label
			action   gtk.Button
			plus     gtk.Button
			minus    gtk.Button
			dialArea gtk.Box
		)
		b.GetObject("main_window").Cast(&win)
		b.GetObject("analog_time_label").Cast(&label)
		b.GetObject("action_button").Cast(&action)
		b.GetObject("plus_button").Cast(&plus)
		b.GetObject("minus_button").Cast(&minus)
		b.GetObject("dial_area").Cast(&dialArea)

		w = &win

		dialObj := gobject.NewObject(gTypeDialWidget, "css-name")
		var dial dialWidget
		dialObj.Cast(&dial)
		dial.Widget.SetHexpand(true)
		dial.Widget.SetVexpand(true)
		dialArea.Append(&dial.Widget)

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

			dialW := (*dialWidget)(unsafe.Pointer(dialObj.GetData(dataKeyGoInstance)))

			dialW.totalSec = totalSec
			dialW.running = running
			dialW.remain = remain
			dial.Widget.QueueDraw()
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

			w, h := float64(dial.Widget.GetWidth()), float64(dial.Widget.GetHeight())
			cx, cy := w/2, h/2
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

			totalSec = intervals * 30

			if paused {
				remain = time.Duration(totalSec) * time.Second
			}

			updateDial()
			updateButtons()
		}

		drag := gtk.NewGestureDrag()
		onDragBegin := func(_ gtk.GestureDrag, x float64, y float64) {
			dragging = true
			handleDialing(x, y)
		}
		drag.ConnectDragBegin(&onDragBegin)
		onDragUpdate := func(drag gtk.GestureDrag, dx float64, dy float64) {
			if dragging {
				var x, y float64
				drag.GetStartPoint(x, y)
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

		dial.Widget.AddController(&drag.EventController)
		dial.Widget.AddController(&click.Gesture.EventController)

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
