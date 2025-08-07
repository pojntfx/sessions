package main

import (
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:generate sh -c "blueprint-compiler batch-compile . . *.blp"
//go:embed ui.ui
var uiXML string

type Timer struct {
	win      *adw.ApplicationWindow
	app      *adw.Application
	dial     *gtk.DrawingArea
	label    *gtk.Label
	action   *gtk.Button
	plus     *gtk.Button
	minus    *gtk.Button
	min      int
	sec      int
	remain   time.Duration
	running  bool
	dragging bool
	timer    glib.SourceHandle
	paused   bool
	angle    float64
	progress float64
}

func main() {
	app := adw.NewApplication("com.github.pojntfx.sessions", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() {
		builder := gtk.NewBuilder()
		builder.AddFromString(uiXML)
		win := adw.NewApplicationWindow(&app.Application)
		win.SetTitle("Sessions")
		win.SetDefaultSize(360, 380)

		t := &Timer{
			win: win,
			app: app,
			min: 5,
		}

		t.dial = builder.GetObject("dial_area").Cast().(*gtk.DrawingArea)
		t.label = builder.GetObject("analog_time_label").Cast().(*gtk.Label)
		t.action = builder.GetObject("action_button").Cast().(*gtk.Button)
		t.plus = builder.GetObject("plus_button").Cast().(*gtk.Button)
		t.minus = builder.GetObject("minus_button").Cast().(*gtk.Button)

		t.setup()

		t.action.ConnectClicked(func() {
			if t.running {
				t.stop()
			} else if t.min > 0 {
				t.start()
			}
		})

		t.plus.ConnectClicked(func() {
			if total := t.min*60 + t.sec; total < 3600 {
				total += 30
				t.min, t.sec = total/60, total%60
				t.update()
			}
		})

		t.minus.ConnectClicked(func() {
			if total := t.min*60 + t.sec; total > 30 {
				total -= 30
				t.min, t.sec = total/60, total%60
				t.update()
			}
		})

		win.SetContent(builder.GetObject("toolbar_view").Cast().(*adw.ToolbarView))
		win.Present()
	})
	app.Run(nil)
}

func (t *Timer) setup() {
	total := t.min*60 + t.sec
	t.progress = float64(total) / 3600.0
	t.angle = t.progress * 2 * math.Pi
	t.dial.SetDrawFunc(t.draw)

	drag := gtk.NewGestureDrag()
	drag.ConnectDragBegin(func(x, y float64) {
		t.dragging = true
		t.interact(x, y)
	})
	drag.ConnectDragUpdate(func(dx, dy float64) {
		if t.dragging {
			x, y, _ := drag.StartPoint()
			t.interact(x+dx, y+dy)
		}
	})
	drag.ConnectDragEnd(func(dx, dy float64) {
		t.dragging = false
		if t.paused {
			t.paused = false
			t.resume()
		} else if !t.running && t.min*60+t.sec > 0 {
			t.start()
		}
	})

	click := gtk.NewGestureClick()
	click.ConnectPressed(func(_ int, x, y float64) {
		t.interact(x, y)
	})

	t.dial.AddController(drag)
	t.dial.AddController(click)
	t.updateButtons()
	t.display()
}

func (t *Timer) updateButtons() {
	if t.running {
		t.action.SetIconName("media-playback-stop-symbolic")
		t.action.SetLabel("Stop")
		t.action.RemoveCSSClass("suggested-action")
		t.action.AddCSSClass("destructive-action")
	} else {
		t.action.SetIconName("media-playback-start-symbolic")
		t.action.SetLabel("Start Timer")
		t.action.RemoveCSSClass("destructive-action")
		t.action.AddCSSClass("suggested-action")
	}
	total := t.min*60 + t.sec
	t.plus.SetSensitive(total < 3600)
	t.minus.SetSensitive(total > 30)
}

func (t *Timer) update() {
	total := t.min*60 + t.sec
	t.progress = float64(total) / 3600.0
	t.angle = t.progress * 2 * math.Pi
	if t.running {
		t.remain = time.Duration(total) * time.Second
	}
	t.display()
	t.updateButtons()
}

func (t *Timer) resume() {
	if t.remain > 0 {
		t.timer = glib.TimeoutAdd(1000, func() bool {
			if !t.running {
				return false
			}
			t.remain -= time.Second
			t.display()
			if t.remain <= 0 {
				t.complete()
				return false
			}
			return true
		})
	}
}

func (t *Timer) interact(x, y float64) {
	if t.running && !t.dragging {
		t.paused = true
		if t.timer > 0 {
			glib.SourceRemove(t.timer)
			t.timer = 0
		}
	}

	w, h := float64(t.dial.Width()), float64(t.dial.Height())
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

	total := intervals * 30
	t.min, t.sec = total/60, total%60
	t.progress = float64(total) / 3600.0
	t.angle = t.progress * 2 * math.Pi

	if t.paused {
		t.remain = time.Duration(total) * time.Second
	}
	t.display()
}

func (t *Timer) display() {
	var text string
	if t.running {
		m, s := int(t.remain.Minutes()), int(t.remain.Seconds())%60
		text = fmt.Sprintf("%02d:%02d", m, s)
	} else {
		if t.sec == 0 {
			text = fmt.Sprintf("%02d:00", t.min)
		} else {
			text = fmt.Sprintf("%02d:%02d", t.min, t.sec)
		}
	}
	t.dial.QueueDraw()
	t.label.SetText(text)
}

func (t *Timer) draw(area *gtk.DrawingArea, cr *cairo.Context, w, h int) {
	cx, cy := float64(w)/2, float64(h)/2
	r := math.Min(cx, cy) - 15

	style := area.StyleContext()
	track := gdk.NewRGBA(0.7, 0.7, 0.7, 1.0)

	accent, found := style.LookupColor("accent_bg_color")
	if !found {
		blue := gdk.NewRGBA(0.2, 0.4, 0.85, 1.0)
		accent = &blue
	}

	err, found := style.LookupColor("error_bg_color")
	if !found {
		if err, found = style.LookupColor("destructive_bg_color"); !found {
			red := gdk.NewRGBA(0.835, 0.196, 0.196, 1.0)
			err = &red
		}
	}

	gray := []float64{float64(track.Red()), float64(track.Green()), float64(track.Blue())}
	blue := []float64{float64(accent.Red()), float64(accent.Green()), float64(accent.Blue())}
	red := []float64{float64(err.Red()), float64(err.Green()), float64(err.Blue())}

	cr.SetSourceRGB(gray[0], gray[1], gray[2])
	cr.SetLineWidth(10)
	cr.Arc(cx, cy, r, 0, 2*math.Pi)
	cr.Stroke()

	total := t.min*60 + t.sec
	if total > 0 {
		end := -math.Pi/2 + 2*math.Pi*t.progress

		if t.running && t.remain > 0 {
			ratio := t.remain.Seconds() / float64(total)
			remain := -math.Pi/2 + 2*math.Pi*t.progress*ratio

			if ratio > 0 {
				cr.SetSourceRGBA(red[0], red[1], red[2], 0.3)
				cr.MoveTo(cx, cy)
				cr.Arc(cx, cy, r, -math.Pi/2, remain)
				cr.LineTo(cx, cy)
				cr.Fill()

				cr.SetSourceRGB(red[0], red[1], red[2])
				cr.SetLineWidth(10)
				cr.SetLineCap(cairo.LineCapRound)
				cr.Arc(cx, cy, r, -math.Pi/2, remain)
				cr.Stroke()
			}
		} else {
			if t.progress > 0 {
				cr.SetSourceRGBA(0.6, 0.6, 0.6, 0.2)
				cr.MoveTo(cx, cy)
				cr.Arc(cx, cy, r, -math.Pi/2, end)
				cr.LineTo(cx, cy)
				cr.Fill()
			}

			cr.SetSourceRGB(blue[0], blue[1], blue[2])
			cr.SetLineWidth(10)
			cr.SetLineCap(cairo.LineCapRound)
			cr.Arc(cx, cy, r, -math.Pi/2, end)
			cr.Stroke()
		}
	}

	var da float64
	var color []float64

	if t.running {
		color = red
		if t.remain > 0 {
			ratio := t.remain.Seconds() / float64(total)
			da = -math.Pi/2 + (t.angle-(-math.Pi/2))*ratio
		} else {
			da = -math.Pi / 2
		}
	} else {
		color = blue
		da = t.angle
	}

	dx := cx + r*math.Cos(da-math.Pi/2)
	dy := cy + r*math.Sin(da-math.Pi/2)

	cr.SetSourceRGB(color[0], color[1], color[2])

	cr.Save()
	cr.Translate(dx, dy)
	cr.Rotate(da - math.Pi/2)

	cr.NewPath()
	cr.Arc(-8, -6, 2, math.Pi, -math.Pi/2)
	cr.Arc(8, -6, 2, -math.Pi/2, 0)
	cr.Arc(8, 6, 2, 0, math.Pi/2)
	cr.Arc(-8, 6, 2, math.Pi/2, math.Pi)
	cr.ClosePath()
	cr.Fill()

	cr.Restore()
}

func (t *Timer) start() {
	t.running = true
	t.remain = time.Duration(t.min*60+t.sec) * time.Second
	t.updateButtons()
	t.display()
	t.timer = glib.TimeoutAdd(1000, func() bool {
		if !t.running {
			return false
		}
		t.remain -= time.Second
		t.display()
		if t.remain <= 0 {
			t.complete()
			return false
		}
		return true
	})
}

func (t *Timer) stop() {
	t.running = false
	if t.timer > 0 {
		glib.SourceRemove(t.timer)
		t.timer = 0
	}
	t.updateButtons()
	t.display()
}

func (t *Timer) complete() {
	t.stop()
	n := gio.NewNotification("Session finished")
	t.app.SendNotification("session-complete", n)
}
