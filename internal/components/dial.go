package components

import (
	"math"
	"runtime"
	"unsafe"

	"codeberg.org/puregotk/purego"
	"codeberg.org/puregotk/puregotk/v4/adw"
	"codeberg.org/puregotk/puregotk/v4/gdk"
	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gobject"
	"codeberg.org/puregotk/puregotk/v4/gobject/types"
	"codeberg.org/puregotk/puregotk/v4/graphene"
	"codeberg.org/puregotk/puregotk/v4/gsk"
	"codeberg.org/puregotk/puregotk/v4/gtk"
)

const (
	signalDialDragBegin = "drag-begin"
	signalDialDragEnd   = "drag-end"

	propertyDialRemainingTime = "remaining-time"
	propertyDialCountingDown  = "counting-down"

	propertyIdDialRemainingTime uint32 = 1
	propertyIdDialCountingDown  uint32 = 2
)

var (
	gTypeDial gobject.Type
)

type Dial struct {
	gtk.Widget

	app           *adw.Application
	remainingTime int
	countingDown  bool
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

	updateRemainingTimeFromPosition := func(x, y float64) {
		remainingTime, ok := v.positionToRemainingTime(x, y)
		if !ok {
			return
		}
		var val gobject.Value
		val.Init(types.GType(gobject.TypeIntVal))
		val.SetInt(int32(remainingTime))
		obj.SetProperty(propertyDialRemainingTime, &val)
		val.Unset()
	}

	drag := gtk.NewGestureDrag()
	onDragBeginCb := func(_ gtk.GestureDrag, x float64, y float64) {
		gobject.SignalEmit(obj, gobject.SignalLookup(signalDialDragBegin, gTypeDial), 0)
		updateRemainingTimeFromPosition(x, y)
	}
	drag.ConnectDragBegin(&onDragBeginCb)
	onDragUpdateCb := func(drag gtk.GestureDrag, dx float64, dy float64) {
		var x, y float64
		drag.GetStartPoint(&x, &y)
		updateRemainingTimeFromPosition(x+dx, y+dy)
	}
	drag.ConnectDragUpdate(&onDragUpdateCb)
	onDragEndCb := func(_ gtk.GestureDrag, dx float64, dy float64) {
		gobject.SignalEmit(obj, gobject.SignalLookup(signalDialDragEnd, gTypeDial), 0)
	}
	drag.ConnectDragEnd(&onDragEndCb)

	click := gtk.NewGestureClick()
	onPress := func(_ gtk.GestureClick, _ int32, x float64, y float64) {
		updateRemainingTimeFromPosition(x, y)
	}
	click.ConnectPressed(&onPress)

	v.Widget.AddController(&drag.EventController)
	v.Widget.AddController(&click.Gesture.EventController)

	return v
}

func (x *Dial) ConnectDragBegin(cb *func()) uint32 {
	cbPtr := uintptr(unsafe.Pointer(cb))
	if cbRefPtr, ok := glib.GetCallback(cbPtr); ok {
		return gobject.SignalConnect(x.GoPointer(), signalDialDragBegin, cbRefPtr)
	}

	fcb := func(_ uintptr) {
		(*cb)()
	}
	cbRefPtr := purego.NewCallback(fcb)
	glib.SaveCallback(cbPtr, cbRefPtr)
	return gobject.SignalConnect(x.GoPointer(), signalDialDragBegin, cbRefPtr)
}

func (x *Dial) ConnectDragEnd(cb *func()) uint32 {
	cbPtr := uintptr(unsafe.Pointer(cb))
	if cbRefPtr, ok := glib.GetCallback(cbPtr); ok {
		return gobject.SignalConnect(x.GoPointer(), signalDialDragEnd, cbRefPtr)
	}

	fcb := func(_ uintptr) {
		(*cb)()
	}
	cbRefPtr := purego.NewCallback(fcb)
	glib.SaveCallback(cbPtr, cbRefPtr)
	return gobject.SignalConnect(x.GoPointer(), signalDialDragEnd, cbRefPtr)
}

func (d *Dial) SetRemainingTime(remainingTime int) {
	var val gobject.Value
	val.Init(types.GType(gobject.TypeIntVal))
	val.SetInt(int32(remainingTime))
	d.SetProperty(propertyDialRemainingTime, &val)
	val.Unset()
}

func (d *Dial) GetRemainingTime() int {
	var val gobject.Value
	val.Init(types.GType(gobject.TypeIntVal))
	d.GetProperty(propertyDialRemainingTime, &val)
	remainingTime := int(val.GetInt())
	val.Unset()
	return remainingTime
}

func (d *Dial) SetCountingDown(countingDown bool) {
	var val gobject.Value
	val.Init(types.GType(gobject.TypeBooleanVal))
	val.SetBoolean(countingDown)
	d.SetProperty(propertyDialCountingDown, &val)
	val.Unset()
}

func (d *Dial) GetCountingDown() bool {
	var val gobject.Value
	val.Init(types.GType(gobject.TypeBooleanVal))
	d.GetProperty(propertyDialCountingDown, &val)
	countingDown := val.GetBoolean()
	val.Unset()
	return countingDown
}

func (d *Dial) positionToRemainingTime(x, y float64) (int, bool) {
	width, height := float64(d.Widget.GetWidth()), float64(d.Widget.GetHeight())
	cx, cy := width/2, height/2
	dx, dy := x-cx, y-cy

	if math.Sqrt(dx*dx+dy*dy) < 15 {
		return 0, false
	}

	a := math.Atan2(dy, dx) + math.Pi/2
	if a < 0 {
		a += 2 * math.Pi
	}

	intervals := int((a / (2 * math.Pi)) * 120)
	if intervals == 0 {
		intervals = 120
	}

	return intervals * int(minDialValue.Seconds()), true
}

func init() {
	var classInit gobject.ClassInitFunc = func(tc *gobject.TypeClass, u uintptr) {
		gobject.SignalNewv(
			signalDialDragBegin,
			gTypeDial,
			gobject.GSignalRunFirstValue,
			nil,
			nil,
			0,
			nil,
			types.GType(gobject.TypeNoneVal),
			0,
			nil,
		)

		gobject.SignalNewv(
			signalDialDragEnd,
			gTypeDial,
			gobject.GSignalRunFirstValue,
			nil,
			nil,
			0,
			nil,
			types.GType(gobject.TypeNoneVal),
			0,
			nil,
		)

		objClass := (*gobject.ObjectClass)(unsafe.Pointer(tc))

		objClass.OverrideSetProperty(func(o *gobject.Object, u uint32, v *gobject.Value, ps *gobject.ParamSpec) {
			w := (*Dial)(unsafe.Pointer(o.GetData(dataKeyGoInstance)))
			switch u {
			case propertyIdDialRemainingTime:
				w.remainingTime = int(v.GetInt())
				w.Widget.QueueDraw()
			case propertyIdDialCountingDown:
				w.countingDown = v.GetBoolean()
				w.Widget.QueueDraw()
			}
		})

		objClass.OverrideGetProperty(func(o *gobject.Object, u uint32, v *gobject.Value, ps *gobject.ParamSpec) {
			w := (*Dial)(unsafe.Pointer(o.GetData(dataKeyGoInstance)))
			switch u {
			case propertyIdDialRemainingTime:
				v.SetInt(int32(w.remainingTime))
			case propertyIdDialCountingDown:
				v.SetBoolean(w.countingDown)
			}
		})

		objClass.InstallProperty(propertyIdDialRemainingTime, gobject.NewParamSpecInt(
			propertyDialRemainingTime,
			"Remaining seconds",
			"Remaining seconds on the dial",
			0,
			int32(maxDialValue.Seconds()),
			300,
			gobject.GParamReadwriteValue,
		))

		objClass.InstallProperty(propertyIdDialCountingDown, gobject.NewParamSpecBoolean(
			propertyDialCountingDown,
			"Counting down",
			"Whether the dial is counting down",
			false,
			gobject.GParamReadwriteValue,
		))

		objClass.OverrideConstructed(func(o *gobject.Object) {
			parentObjClass := (*gobject.ObjectClass)(unsafe.Pointer(tc.PeekParent()))
			parentObjClass.GetConstructed()(o)

			var parent gtk.Widget
			o.Cast(&parent)

			w := &Dial{
				Widget:        parent,
				remainingTime: 300,
				countingDown:  false,
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
			styleContext.LookupColor("error_color", &errColor)

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

			if dialW.remainingTime > 0 || dialW.countingDown {
				progress := float64(dialW.remainingTime) / maxDialValue.Seconds()
				var angle float64
				var lineColor gdk.RGBA
				var fillR, fillG, fillB, fillA float32
				noFill := false

				if dialW.countingDown && dialW.remainingTime > 0 {
					angle = -math.Pi/2 + 2*math.Pi*progress
					lineColor = errColor
					fillR, fillG, fillB, fillA = errColor.Red, errColor.Green, errColor.Blue, 0.3
				} else if dialW.countingDown && dialW.remainingTime == 0 {
					angle = -math.Pi / 2
					lineColor = errColor
					noFill = true
				} else {
					angle = -math.Pi/2 + 2*math.Pi*progress
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

				isFullCircle := startX == endX && startY == endY
				largeArc := angle+math.Pi/2 > math.Pi

				drawArcOrCircle := func(builder *gsk.PathBuilder, rx, ry float32, endX, endY float32) {
					if isFullCircle {
						builder.AddCircle(&graphene.Point{X: float32(cx), Y: float32(cy)}, rx)
					} else {
						builder.SvgArcTo(rx, ry, 0, largeArc, true, endX, endY)
					}
				}

				if !noFill {
					if isFullCircle {
						fullFillBuilder := gsk.NewPathBuilder()
						defer fullFillBuilder.Unref()
						fullFillBuilder.AddCircle(&graphene.Point{X: float32(cx), Y: float32(cy)}, float32(r))
						fullFillPath := fullFillBuilder.ToPath()
						defer fullFillPath.Unref()
						snapshot.AppendFill(fullFillPath, gsk.FillRuleWindingValue, &fillColor)
						strokeBorder(fullFillPath)
					} else {
						arcBuilder := gsk.NewPathBuilder()
						defer arcBuilder.Unref()
						arcBuilder.MoveTo(float32(cx), float32(cy))
						arcBuilder.LineTo(startX, startY)
						drawArcOrCircle(arcBuilder, float32(r), float32(r), endX, endY)
						arcBuilder.LineTo(float32(cx), float32(cy))
						arcPath := arcBuilder.ToPath()
						defer arcPath.Unref()
						snapshot.AppendFill(arcPath, gsk.FillRuleWindingValue, &fillColor)
						strokeBorder(arcPath)
					}
				}

				lineStrokeColor := gdk.RGBA{
					Red:   lineColor.Red,
					Green: lineColor.Green,
					Blue:  lineColor.Blue,
					Alpha: 1.0,
				}

				arcLineBuilder := gsk.NewPathBuilder()
				defer arcLineBuilder.Unref()
				arcLineBuilder.MoveTo(startX, startY)
				drawArcOrCircle(arcLineBuilder, float32(r), float32(r), endX, endY)
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
					drawArcOrCircle(outerArcBuilder, float32(r)+5, float32(r)+5, float32(cx+(r+5)*math.Sin(angle+math.Pi/2)), float32(cy-(r+5)*math.Cos(angle+math.Pi/2)))
					outerArcPath := outerArcBuilder.ToPath()
					defer outerArcPath.Unref()
					strokeBorder(outerArcPath)

					innerArcBuilder := gsk.NewPathBuilder()
					defer innerArcBuilder.Unref()
					innerArcBuilder.MoveTo(float32(cx), float32(cy-(r-5)))
					drawArcOrCircle(innerArcBuilder, float32(r)-5, float32(r)-5, float32(cx+(r-5)*math.Sin(angle+math.Pi/2)), float32(cy-(r-5)*math.Cos(angle+math.Pi/2)))
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
		parentQuery.InstanceSize,
		&instanceInit,
		0,
	)
}
