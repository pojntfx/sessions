package main

import (
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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

		var (
			w      = b.GetObject("main_window").Cast().(*adw.Window)
			dial   = b.GetObject("dial_area").Cast().(*gtk.DrawingArea)
			label  = b.GetObject("analog_time_label").Cast().(*gtk.Label)
			action = b.GetObject("action_button").Cast().(*gtk.Button)
			plus   = b.GetObject("plus_button").Cast().(*gtk.Button)
			minus  = b.GetObject("minus_button").Cast().(*gtk.Button)
		)

		var (
			totalSec = 300
			running  = false
		)
		updateButtons := func() {
			if running {
				action.SetIconName("media-playback-stop-symbolic")
				action.SetLabel("Stop")
				action.RemoveCSSClass("suggested-action")
				action.AddCSSClass("destructive-action")
			} else {
				action.SetIconName("media-playback-start-symbolic")
				action.SetLabel("Start Timer")
				action.RemoveCSSClass("destructive-action")
				action.AddCSSClass("suggested-action")
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
			dial.QueueDraw()
		}

		var (
			timer = glib.SourceHandle(0)
		)
		createSessionFinishedHandler := func() func() bool {
			return func() bool {
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

					n := gio.NewNotification("Session finished")
					a.SendNotification("session-finished", n)

					return false
				}

				return true
			}
		}

		startTimer := func() {
			running = true
			remain = time.Duration(totalSec) * time.Second

			updateButtons()
			updateDial()

			timer = glib.TimeoutAdd(1000, createSessionFinishedHandler())
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
				timer = glib.TimeoutAdd(1000, createSessionFinishedHandler())
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

			w, h := float64(dial.Width()), float64(dial.Height())
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
		}

		dial.SetDrawFunc(func(area *gtk.DrawingArea, cr *cairo.Context, w, h int) {
			cx, cy := float64(w)/2, float64(h)/2
			r := math.Min(cx, cy) - 15

			style := area.StyleContext()
			accent, _ := style.LookupColor("accent_bg_color")
			err, _ := style.LookupColor("error_bg_color")
			cr.SetSourceRGB(0.7, 0.7, 0.7)
			cr.SetLineWidth(10)
			cr.Arc(cx, cy, r, 0, 2*math.Pi)
			cr.Stroke()

			if totalSec > 0 {
				progress := float64(totalSec) / 3600.0
				end := -math.Pi/2 + 2*math.Pi*progress
				var (
					handleAngle                float64
					handleColor                *gdk.RGBA
					angle                      float64
					lineColor                  *gdk.RGBA
					fillR, fillG, fillB, fillA float64
				)

				if running && remain > 0 {
					ratio := remain.Seconds() / float64(totalSec)
					angle = -math.Pi/2 + 2*math.Pi*progress*ratio
					lineColor = err
					fillR, fillG, fillB, fillA = float64(err.Red()), float64(err.Green()), float64(err.Blue()), 0.3
				} else {
					angle = end
					lineColor = accent
					fillR, fillG, fillB, fillA = 0.6, 0.6, 0.6, 0.2
				}

				cr.SetSourceRGBA(fillR, fillG, fillB, fillA)
				cr.MoveTo(cx, cy)
				cr.Arc(cx, cy, r, -math.Pi/2, angle)
				cr.LineTo(cx, cy)
				cr.Fill()

				cr.SetSourceRGB(float64(lineColor.Red()), float64(lineColor.Green()), float64(lineColor.Blue()))
				cr.SetLineWidth(10)
				cr.SetLineCap(cairo.LineCapRound)
				cr.Arc(cx, cy, r, -math.Pi/2, angle)
				cr.Stroke()

				handleAngle = angle
				handleColor = lineColor

				dx := cx + r*math.Cos(handleAngle)
				dy := cy + r*math.Sin(handleAngle)
				cr.SetSourceRGB(float64(handleColor.Red()), float64(handleColor.Green()), float64(handleColor.Blue()))
				cr.Save()
				cr.Translate(dx, dy)
				cr.Arc(0, 0, 8, 0, 2*math.Pi)
				cr.Fill()
				cr.Restore()
			}
		})

		drag := gtk.NewGestureDrag()
		drag.ConnectDragBegin(func(x, y float64) {
			dragging = true
			handleDialing(x, y)
		})
		drag.ConnectDragUpdate(func(dx, dy float64) {
			if dragging {
				x, y, _ := drag.StartPoint()
				handleDialing(x+dx, y+dy)
			}
		})
		drag.ConnectDragEnd(func(dx, dy float64) {
			dragging = false

			if paused {
				paused = false
				resumeTimer()
			} else if !running && totalSec > 0 {
				startTimer()
			}
		})

		click := gtk.NewGestureClick()
		click.ConnectPressed(func(_ int, x, y float64) {
			handleDialing(x, y)
		})

		dial.AddController(drag)
		dial.AddController(click)

		toggleTimerAction := gio.NewSimpleAction("toggleTimer", nil)
		toggleTimerAction.ConnectActivate(func(parameter *glib.Variant) {
			if running {
				stopTimer()
			} else if totalSec > 0 {
				startTimer()
			}
		})
		a.AddAction(toggleTimerAction)

		addTimeAction := gio.NewSimpleAction("addTime", nil)
		addTimeAction.ConnectActivate(func(parameter *glib.Variant) {
			if totalSec < 3600 {
				totalSec += 30
				if running {
					remain = time.Duration(totalSec) * time.Second
				}

				updateDial()
				updateButtons()
			}
		})
		a.AddAction(addTimeAction)

		removeTimeAction := gio.NewSimpleAction("removeTime", nil)
		removeTimeAction.ConnectActivate(func(parameter *glib.Variant) {
			if totalSec > 30 {
				totalSec -= 30
				if running {
					remain = time.Duration(totalSec) * time.Second
				}

				updateDial()
				updateButtons()
			}
		})
		a.AddAction(removeTimeAction)

		openAboutAction := gio.NewSimpleAction("openAbout", nil)
		openAboutAction.ConnectActivate(func(parameter *glib.Variant) {
			aboutDialog.Present(&w.Window)
		})
		a.AddAction(openAboutAction)

		updateButtons()
		updateDial()

		a.AddWindow(&w.Window)
	})

	if code := a.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}
