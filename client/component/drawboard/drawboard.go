package drawboard

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/iafan/goplayspace/client/draw"
	"github.com/iafan/goplayspace/client/js/canvas"
	"github.com/iafan/goplayspace/client/js/document"
	"github.com/iafan/goplayspace/client/js/window"
	"github.com/iafan/goplayspace/client/util"
)

const (
	firstStepDelay = 500 * time.Millisecond
	stepDelay      = 500 * time.Millisecond
	// should be longer than `.say-bubble.animate`` CSS animation duration
	removeBubbleDelay = 5 * time.Second

	// when determining the scale of the board, how many cells should be visible
	// in each direction from the center of the board; the scale is calculated
	// based on the smallest dimension (width or height)
	stepsInEachDirection = 15

	walkFrameDistance = 2                    // distance in px along the path between animation frames
	walkFrames        = 5                    // total frames in the walk animation
	rotationFrame     = (walkFrames - 1) / 2 // middle frame index
	virtualWalkFrames = walkFrames*2 - 1     // we move back-forth between frames rather than cycle
	walkFrameSize     = 50

	boardLineWidth    = 1
	boardStrokeStyle  = "rgba(0, 0, 0, 0.05)"
	fifthStrokeStyle  = "rgba(0, 0, 0, 0.09)"
	centerStrokeStyle = "rgba(0, 0, 0, 0.16)"
)

var (
	colors = []string{
		"original",
		"periwinkle",
		"yellow",
		"red",
		"orange",
		"lime-green",
		"forest-green",
		"purple",
		"gray",
		"brown",
		"fuschia",
		"hot-pink",
	}
)

type actor struct {
	ctx    *canvas.CanvasRenderingContext2D
	gopher *js.Object

	startX     float64
	startY     float64
	startAngle float64

	targetX     float64
	targetY     float64
	targetAngle float64
	targetDist  float64

	startTime  time.Time
	targetTime time.Time

	x, y     float64
	angle    float64
	color    string
	width    float64
	initialX float64
	initialY float64

	Actions draw.Actor `vecty:"prop"`

	step int
}

// DrawBoard represents the drawing board with animation logic
type DrawBoard struct {
	vecty.Core
	canvas          *canvas.Canvas
	canvasWrapper   *js.Object
	initialized     bool
	ctx             *canvas.CanvasRenderingContext2D
	connectedActors map[string]*actor
	actors          draw.ActorsList

	accelerate bool
	tabDown    bool

	w, h     float64
	stepSize float64
}

func New(aa draw.ActorsList) *DrawBoard {
	// var actors []*actor
	// for _, list := range aa {
	// 	actors = append(actors, &actor{
	// 		Actions: list,
	// 	})
	// }

	fmt.Println("New Drawboard")

	return &DrawBoard{
		connectedActors: make(map[string]*actor),
		actors:          aa,
	}
}

func (b *DrawBoard) pollForActors() {
	rand.Seed(time.Now().Unix())

	for {
		select {
		case <-time.After(time.Second):
			if b.actors == nil {
				continue
			}

			fmt.Println("Checking for more actors")
			maybeNewActors := b.actors.Actors()
			for _, newActor := range maybeNewActors {
				id := newActor.ID()
				if _, ok := b.connectedActors[id]; ok {
					continue
				}

				color := colors[rand.Intn(len(colors))]

				elemID := "gopher" + id
				el := document.CreateElement("div")
				el.Set("id", elemID)
				el.Set("className", "gopher gopher-"+color)
				b.canvasWrapper.Call("appendChild", el)

				spawnableW := int(b.w * 0.6)
				spawnableH := int(b.h * 0.6)

				randomX := rand.Intn(spawnableW) - (spawnableW / 2)
				randomY := rand.Intn(spawnableH) - (spawnableH / 2)

				fmt.Println("rand", randomX, randomY)

				na := &actor{
					Actions:  newActor,
					ctx:      b.canvas.GetContext2D(),
					gopher:   document.QuerySelector("#gopher" + id),
					initialX: float64(randomX),
					initialY: float64(randomY),
				}

				style := fmt.Sprintf(
					"transform: translateX(%.2fpx) translateY(%.2fpx);",
					na.initialX, na.initialY,
				)

				na.gopher.Call("setAttribute", "style", style)

				go na.animate(b)

				b.connectedActors[id] = na
			}
		}
	}

}

func (b *DrawBoard) getDOMNodes() {
	if b.canvas == nil {
		c := document.QuerySelector("canvas")
		if c != nil {
			b.canvas = &canvas.Canvas{c}
			b.ctx = b.canvas.GetContext2D()
			go b.pollForActors()
		}
		b.canvasWrapper = document.QuerySelector(".canvas-wrapper")
	}
}

func (b *DrawBoard) renderBoardLines() {
	cX := b.w / 2
	cY := b.h / 2

	nX := int(cX/b.stepSize) + 1
	nY := int(cY/b.stepSize) + 1

	b.ctx.SetLineWidth(boardLineWidth)

	for x := -nX; x <= nX; x++ {
		b.ctx.SetStrokeStyle(boardStrokeStyle)
		if x%5 == 0 {
			b.ctx.SetStrokeStyle(fifthStrokeStyle)
		}
		if x == 0 {
			b.ctx.SetStrokeStyle(centerStrokeStyle)
		}
		b.ctx.BeginPath()
		b.ctx.MoveTo(cX+float64(x)*b.stepSize, 0)
		b.ctx.LineTo(cX+float64(x)*b.stepSize, b.h)
		b.ctx.Stroke()
	}

	for y := -nY; y <= nY; y++ {
		b.ctx.SetStrokeStyle(boardStrokeStyle)
		if y%5 == 0 {
			b.ctx.SetStrokeStyle(fifthStrokeStyle)
		}
		if y == 0 {
			b.ctx.SetStrokeStyle(centerStrokeStyle)
		}
		b.ctx.BeginPath()
		b.ctx.MoveTo(0, cY+float64(y)*b.stepSize)
		b.ctx.LineTo(b.w, cY+float64(y)*b.stepSize)
		b.ctx.Stroke()
	}
}

// addSpeechBubble shows the animated 'speech bubble'
// x, y are the center coordinates of the bubble in pixels
// relative to the center of the board
func (b *DrawBoard) addSpeechBubble(x, y float64, s string) {
	el := document.CreateElement("div")
	el.Set("className", "say-bubble")

	el.Set("innerHTML", s)
	b.canvasWrapper.Call("appendChild", el)

	// need to wait for the element to be rendered
	// in order to get offsetWidth / offsetHeight for centering
	util.Schedule(func() {
		elw := el.Get("offsetWidth").Float()
		elh := el.Get("offsetHeight").Float()

		cX := b.w / 2
		cY := b.h / 2

		// center the bubble around x, y point
		style := fmt.Sprintf(
			"left: %.0fpx; top: %.0fpx",
			cX+x-elw/2, cY+y-elh/2,
		)
		el.Call("setAttribute", "style", style)

		// start animation
		el.Set("className", "say-bubble animate")

		time.AfterFunc(removeBubbleDelay, func() {
			b.canvasWrapper.Call("removeChild", el)
		})
	})
}

func (b *actor) doSubStep(db *DrawBoard, pos float64) {
	// b.startX = b.startX + 5
	// b.startY = b.startY + 5
	// b.targetX = b.targetX + 5
	// b.targetY = b.targetY + 5

	oldX := b.x
	oldY := b.y

	b.x = (b.targetX-b.startX)*pos + b.startX
	b.y = (b.targetY-b.startY)*pos + b.startY
	b.angle = (b.targetAngle-b.startAngle)*pos + b.startAngle

	// b.x = b.x + b.initialX
	// b.y = b.y + b.initialY
	//console.Log("x:", b.x, "y:", b.y, "angle:", b.angle)

	cX := (db.w / 2) + b.initialX
	cY := (db.h / 2) + b.initialY

	if b.color != "" {
		b.ctx.SetLineWidth(b.width)
		b.ctx.SetStrokeStyle(b.color)
		b.ctx.BeginPath()
		b.ctx.MoveTo(cX+oldX, cY+oldY)
		b.ctx.LineTo(cX+b.x, cY+b.y)
		b.ctx.Stroke()
	}

	frame := int(b.targetDist*pos/walkFrameDistance) % virtualWalkFrames

	// offset frame number by rotationFrame index
	frame = (frame + rotationFrame) % virtualWalkFrames

	if frame > walkFrames-1 {
		frame = virtualWalkFrames - frame
	}

	bgPos := -frame * walkFrameSize

	style := fmt.Sprintf(
		"transform: translateX(%.2fpx) translateY(%.2fpx) rotate(%.2fdeg); "+
			"background-position-x: %dpx;",
		b.x+b.initialX, b.y+b.initialY, b.angle,
		bgPos,
	)

	b.gopher.Call("setAttribute", "style", style)
}

func (b *actor) doStep(db *DrawBoard) {
	t := time.Now()

	if b.targetTime.IsZero() || b.targetTime.Sub(t) <= 0 || db.accelerate {
		b.doSubStep(db, 1)

		// new step
		b.step = b.step + 1

		b.startX = b.x
		b.startY = b.y
		b.startAngle = b.angle

		b.startTime = t
		b.targetTime = t

		a, ok := b.Actions.Next()
		if !ok {
			return
		}

		delay := stepDelay

		switch a.Kind {
		case draw.Step:
			b.targetTime = t.Add(time.Duration(float64(delay) * a.FVal))

			rad := (-90 + b.angle) * 2 * math.Pi / 360
			b.targetX = b.startX + math.Cos(rad)*db.stepSize*a.FVal
			b.targetY = b.startY + math.Sin(rad)*db.stepSize*a.FVal

			// stop accelerating only after the 'Step' event; accelerate through others
			if db.tabDown {
				db.accelerate = false
			}

		case draw.Left:
			b.targetTime = t.Add(delay)
			b.targetAngle = b.startAngle - a.FVal // sign inverted to match clock-wise CSS rotation
		case draw.Right:
			b.targetTime = t.Add(delay)
			b.targetAngle = b.startAngle + a.FVal // sign inverted to match clock-wise CSS rotation
		case draw.Color:
			b.color = a.SVal
			util.Schedule(func() { b.doStep(db) })
			return
		case draw.Width:
			b.width = a.FVal
			util.Schedule(func() { b.doStep(db) })
			return
		case draw.Say:
			db.addSpeechBubble(b.x+b.initialX, b.y+b.initialY, a.SVal)
			util.Schedule(func() { b.doStep(db) })
			return
		}

		b.targetDist = math.Sqrt(
			math.Pow(b.targetX-b.startX, 2) + math.Pow(b.targetY-b.startY, 2),
		)
	}

	// calculate current position
	total := b.targetTime.Sub(b.startTime)  // total duration
	passed := t.Sub(b.startTime)            // passed duration
	rel := float64(passed) / float64(total) // passed [0..1]
	b.doSubStep(db, rel)

	window.RequestAnimationFrame(func() { b.doStep(db) })
}

func (b *actor) animate(db *DrawBoard) {
	for {
		select {
		case <-time.After(time.Second):
			fmt.Println("Animating")
			// db.getDOMNodes()

			// set defaults
			b.width = 2

			b.step = -1
			//console.Log("Animation started")
			time.AfterFunc(firstStepDelay, func() {
				b.doStep(db)
			})
		}
	}
}

func (b *DrawBoard) onRendered() {
	b.getDOMNodes()

	time.AfterFunc(100*time.Millisecond, func() {
		document.QuerySelector(".canvas-lightbox").Call("focus")
	})

	if !b.initialized {
		b.initialized = true
		window.AddEventListener("resize", b.onResize)
		b.onResize()

		// start the animation
		// for k, actor := range b.connectedActors {
		// 	fmt.Println("Animating", k)
		// 	actor.animate(b)
		// }

		// t := time.Now()
		// b.startTime = t
		// b.targetTime = t.Add(stepDelay)
	}
}

func (b *DrawBoard) handleKeyDown(e *vecty.Event) {
	switch e.Get("key").String() {
	case "Shift":
		b.accelerate = true
	case "Tab":
		e.Call("preventDefault")
		if b.tabDown {
			return
		}
		b.accelerate = true
		b.tabDown = true
	default:
		//console.Log(e.Get("key").String())
	}
}

func (b *DrawBoard) handleKeyUp(e *vecty.Event) {
	switch e.Get("key").String() {
	case "Shift":
		b.accelerate = false
	case "Tab":
		e.Call("preventDefault")
		if !b.tabDown {
			return
		}
		b.tabDown = false
	}
}

func (b *DrawBoard) onResize() {
	b.w, b.h = b.canvas.GetNodeSize()
	min := b.w
	if b.h < min {
		min = b.h
	}
	b.stepSize = min / (stepsInEachDirection*2 + 1) // "+1" to add 0.5 steps around
	b.canvas.SetSize(b.w, b.h)
	b.renderBoardLines()
}

// SkipRender implements the vecty.Component interface.
func (b *DrawBoard) SkipRender(prev vecty.Component) bool {
	return true
}

// Render implements the vecty.Component interface.
func (b *DrawBoard) Render() vecty.ComponentOrHTML {
	util.Schedule(b.onRendered)

	elems := []vecty.MarkupOrChild{
		vecty.Markup(
			vecty.Class("canvas-wrapper"),
		),
		elem.Canvas(),
	}

	return elem.Div(
		vecty.Markup(
			vecty.Class("canvas-lightbox"),
			vecty.Attribute("tabindex", 0),
			event.KeyDown(b.handleKeyDown),
			event.KeyUp(b.handleKeyUp),
		),
		elem.Div(elems...),
	)
}
