package components

import (
	"math"
	"runtime"
	"time"
	"unsafe"

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

	totalSec int
	running  bool
	remain   time.Duration
}

func NewDial(FirstPropertyNameVar string, varArgs ...interface{}) Dial {
	obj := gobject.NewObject(gTypeDial, FirstPropertyNameVar, varArgs...)

	var v Dial
	obj.Cast(&v)

	return v
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

	gTypeDial = gobject.TypeRegisterStaticSimple(
		parentQuery.Type,
		"Dial",
		parentQuery.ClassSize,
		&classInit,
		parentQuery.InstanceSize+uint(unsafe.Sizeof(Dial{}))+uint(unsafe.Sizeof(&Dial{})),
		&instanceInit,
		0,
	)
}
