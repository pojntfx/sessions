package components

import (
	"math"
	"runtime"
	"time"
	"unsafe"

	"github.com/jwijenbergh/puregotk/v4/adw"
	"github.com/jwijenbergh/puregotk/v4/gdk"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gobject"
	"github.com/jwijenbergh/puregotk/v4/graphene"
	"github.com/jwijenbergh/puregotk/v4/gsk"
	"github.com/jwijenbergh/puregotk/v4/gtk"
)

var (
	gTypeDial gobject.Type
)

type Dial struct {
	gtk.Widget

	app      *adw.Application
	totalSec int
	running  bool
	remain   time.Duration
}

func NewDial(app *adw.Application, FirstPropertyNameVar string, varArgs ...interface{}) Dial {
	obj := gobject.NewObject(gTypeDial, FirstPropertyNameVar, varArgs...)

	var v Dial
	obj.Cast(&v)

	dial := (*Dial)(unsafe.Pointer(obj.GetData(dataKeyGoInstance)))
	dial.app = app

	var styleChangedCallback func(gobject.Object, uintptr) = func(_ gobject.Object, _ uintptr) {
		v.Widget.QueueDraw()
	}
	app.GetStyleManager().ConnectNotify(&styleChangedCallback)

	return v
}

func (d *Dial) SetTimer(totalSec int, running bool, remain time.Duration) {
	dialW := (*Dial)(unsafe.Pointer(d.Widget.GetData(dataKeyGoInstance)))

	dialW.totalSec = totalSec
	dialW.running = running
	dialW.remain = remain

	d.Widget.QueueDraw()
}

func init() {
	var classInit gobject.ClassInitFunc = func(tc *gobject.TypeClass, u uintptr) {
		objClass := (*gobject.ObjectClass)(unsafe.Pointer(tc))

		objClass.OverrideConstructed(func(o *gobject.Object) {
			parentObjClass := (*gobject.ObjectClass)(unsafe.Pointer(tc.PeekParent()))
			parentObjClass.GetConstructed()(o)

			var parent gtk.Widget
			o.Cast(&parent)

			w := &Dial{
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
			dialW := (*Dial)(unsafe.Pointer(widget.GetData(dataKeyGoInstance)))
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

			highContrast := dialW.app.GetStyleManager().GetHighContrast()

			var borderColor gdk.RGBA
			if highContrast {
				styleContext.LookupColor("window_fg_color", &borderColor)
				borderColor.Alpha = 0.5
			}

			strokeBorder := func(path *gsk.Path) {
				if highContrast {
					stroke := gsk.NewStroke(1.0)
					defer stroke.Free()
					snapshot.AppendStroke(path, stroke, &borderColor)
				}
			}

			grayColor := gdk.RGBA{
				Red:   0.7,
				Green: 0.7,
				Blue:  0.7,
				Alpha: 1.0,
			}

			fullCircleBuilder := gsk.NewPathBuilder()
			defer fullCircleBuilder.Unref()
			centerPoint := graphene.Point{X: float32(cx), Y: float32(cy)}
			fullCircleBuilder.AddCircle(&centerPoint, float32(r))
			fullCirclePath := fullCircleBuilder.ToPath()
			defer fullCirclePath.Unref()
			fullCircleStroke := gsk.NewStroke(10.0)
			defer fullCircleStroke.Free()
			snapshot.AppendStroke(fullCirclePath, fullCircleStroke, &grayColor)

			if highContrast {
				outerBorderBuilder := gsk.NewPathBuilder()
				defer outerBorderBuilder.Unref()
				outerBorderBuilder.AddCircle(&centerPoint, float32(r)+5)
				outerBorderPath := outerBorderBuilder.ToPath()
				defer outerBorderPath.Unref()
				strokeBorder(outerBorderPath)

				innerBorderBuilder := gsk.NewPathBuilder()
				defer innerBorderBuilder.Unref()
				innerBorderBuilder.AddCircle(&centerPoint, float32(r)-5)
				innerBorderPath := innerBorderBuilder.ToPath()
				defer innerBorderPath.Unref()
				strokeBorder(innerBorderPath)
			}

			if dialW.totalSec > 0 {
				progress := float64(dialW.totalSec) / maxDialValue
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
				defer arcBuilder.Unref()
				arcBuilder.MoveTo(float32(cx), float32(cy))
				arcBuilder.LineTo(startX, startY)
				arcBuilder.SvgArcTo(float32(r), float32(r), 0, angle+math.Pi/2 > math.Pi, true, endX, endY)
				arcBuilder.LineTo(float32(cx), float32(cy))
				arcPath := arcBuilder.ToPath()
				defer arcPath.Unref()
				snapshot.AppendFill(arcPath, gsk.FillRuleWindingValue, &fillColor)
				strokeBorder(arcPath)

				lineStrokeColor := gdk.RGBA{
					Red:   lineColor.Red,
					Green: lineColor.Green,
					Blue:  lineColor.Blue,
					Alpha: 1.0,
				}

				arcLineBuilder := gsk.NewPathBuilder()
				defer arcLineBuilder.Unref()
				arcLineBuilder.MoveTo(startX, startY)
				arcLineBuilder.SvgArcTo(float32(r), float32(r), 0, angle+math.Pi/2 > math.Pi, true, endX, endY)
				arcLinePath := arcLineBuilder.ToPath()
				defer arcLinePath.Unref()
				arcStroke := gsk.NewStroke(10.0)
				defer arcStroke.Free()
				arcStroke.SetLineCap(gsk.LineCapRoundValue)
				snapshot.AppendStroke(arcLinePath, arcStroke, &lineStrokeColor)

				if highContrast {
					startCapBuilder := gsk.NewPathBuilder()
					defer startCapBuilder.Unref()
					startCapBuilder.MoveTo(float32(cx), float32(cy-(r+5)))
					startCapBuilder.SvgArcTo(5, 5, 0, false, false, float32(cx), float32(cy-(r-5)))
					startCapPath := startCapBuilder.ToPath()
					defer startCapPath.Unref()
					strokeBorder(startCapPath)

					outerArcBuilder := gsk.NewPathBuilder()
					defer outerArcBuilder.Unref()
					outerArcBuilder.MoveTo(float32(cx), float32(cy-(r+5)))
					outerArcBuilder.SvgArcTo(float32(r)+5, float32(r)+5, 0, angle+math.Pi/2 > math.Pi, true, float32(cx+(r+5)*math.Sin(angle+math.Pi/2)), float32(cy-(r+5)*math.Cos(angle+math.Pi/2)))
					outerArcPath := outerArcBuilder.ToPath()
					defer outerArcPath.Unref()
					strokeBorder(outerArcPath)

					innerArcBuilder := gsk.NewPathBuilder()
					defer innerArcBuilder.Unref()
					innerArcBuilder.MoveTo(float32(cx), float32(cy-(r-5)))
					innerArcBuilder.SvgArcTo(float32(r)-5, float32(r)-5, 0, angle+math.Pi/2 > math.Pi, true, float32(cx+(r-5)*math.Sin(angle+math.Pi/2)), float32(cy-(r-5)*math.Cos(angle+math.Pi/2)))
					innerArcPath := innerArcBuilder.ToPath()
					defer innerArcPath.Unref()
					strokeBorder(innerArcPath)
				}

				handleX := float32(cx + r*math.Cos(angle))
				handleY := float32(cy + r*math.Sin(angle))

				handleBuilder := gsk.NewPathBuilder()
				defer handleBuilder.Unref()
				handlePoint := graphene.Point{X: handleX, Y: handleY}
				handleBuilder.AddCircle(&handlePoint, 8)
				handlePath := handleBuilder.ToPath()
				defer handlePath.Unref()
				snapshot.AppendFill(handlePath, gsk.FillRuleWindingValue, &lineColor)
				strokeBorder(handlePath)
			}
		})
	}

	var instanceInit gobject.InstanceInitFunc = func(ti *gobject.TypeInstance, tc *gobject.TypeClass) {}

	var parentQuery gobject.TypeQuery
	gobject.NewTypeQuery(gtk.WidgetGLibType(), &parentQuery)

	gTypeDial = gobject.TypeRegisterStaticSimple(
		parentQuery.Type,
		"SessionsDial",
		parentQuery.ClassSize,
		&classInit,
		parentQuery.InstanceSize+uint(unsafe.Sizeof(Dial{}))+uint(unsafe.Sizeof(&Dial{})),
		&instanceInit,
		0,
	)
}
