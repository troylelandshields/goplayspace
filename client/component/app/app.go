package app

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/iafan/goplayspace/client/component/drawboard"
	"github.com/iafan/goplayspace/client/hash"
	"github.com/iafan/goplayspace/client/util"
	"honnef.co/go/js/xhr"
)

// Application implements the main application view
type Application struct {
	vecty.Core

	Hash      *hash.Hash
	snippetID string

	isLoading     bool
	isDrawingMode bool
	needRender    bool

	// Log properties
	hasRun bool

	// Draw mode properties
	DrawBoard *drawboard.DrawBoard
}

func (a *Application) rerenderIfNeeded() {
	if !a.needRender {
		return
	}
	a.needRender = false
	vecty.Rerender(a)
}

func (a *Application) wantRerender(reason string) {
	//console.Log("want rerender:", reason)
	a.needRender = true
	util.Schedule(a.rerenderIfNeeded)
}

var compileErrorLineExtractorR = regexp.MustCompile(`\/main\.go:(\d+):\s`)
var fmtErrorLineExtractorR = regexp.MustCompile(`(?m)^(\d+):(\d+):\s`)

var domMonitorInterval = 5 * time.Millisecond

func (a *Application) doRun() {
	//a.doFormat()
	go a.doRunAsync()
}

func (a *Application) doRunAsync() {
	defer a.doRunAsyncComplete()

	a.hasRun = true
	a.isDrawingMode = true
}

func (a *Application) doRunAsyncComplete() {
	a.wantRerender("doRunAsyncComplete")
	// util.Schedule(func() { a.log.ScrollToBottom() })
}

func (a *Application) onHashChange(h *hash.Hash) {
	defer a.wantRerender("onHashChange")

	if a.isLoading || h.ID == "" {
		return
	}

	// if a.updateStateFromHash(h) {
	// 	a.doLoad(h.ID)
	// }
}

func (a *Application) doLoad(id string) {
	if id == a.snippetID || id == "" {
		return
	}
	a.isLoading = true
	go a.doLoadAsync(id)
}

func (a *Application) doLoadAsync(id string) {
	defer a.doLoadAsyncComplete(id)

	req := xhr.NewRequest("GET", "/load?"+id)
	err := req.Send(nil)
	//bodyBytes, err := xhr.Send("GET", "/load?"+id, nil)
	if err != nil {
		// a.err = err.Error()
		return
	}
	if req.Status != 200 {
		// a.err = req.ResponseText
		return
	}

	// setting new text will cause OnChange event,
	// and hash will be reset; so update it afterwards
	a.Hash.ID = id
}

func (a *Application) doLoadAsyncComplete(id string) {
	a.isLoading = false
	a.snippetID = id
	a.wantRerender("doLoadAsyncComplete")
}

// Mount implements the vecty.Mounter interface.
func (a *Application) Mount() {
	switch a.Hash.ID {
	case "":
	default:
		a.onHashChange(a.Hash)
	}
}

// Unmount implements the vecty.Unmounter interface.
func (a *Application) Unmount() {
}

// Render renders the application
func (a *Application) Render() vecty.ComponentOrHTML {
	defer a.doRun()
	//console.Time("app:render")
	//defer console.TimeEnd("app:render")
	fmt.Println("Rendering app")

	if a.Hash == nil {
		a.Hash = hash.New(a.onHashChange)
		// a.updateStateFromHash(a.Hash)
	}

	return elem.Body(
		vecty.Markup(
			vecty.MarkupIf(util.IsSafari(), vecty.Class("safari")),
			vecty.MarkupIf(util.IsIOS(), vecty.Class("ios")),
			vecty.MarkupIf(a.isDrawingMode, vecty.Class("drawingmode")),
		),
		elem.Div(
			vecty.Markup(
				vecty.Class("header"),
			),
		),
		// elem.Div(
		// 	vecty.Markup(
		// 		vecty.Class("body-wrapper"),
		// 	),
		// ),
		elem.Div(
			vecty.Markup(
				vecty.Class("body-wrapper"),
			),
			elem.Div(
				vecty.Markup(
					vecty.Class("content-wrapper"),
				),
				// a.editor,
			),
			elem.Div(
				vecty.Markup(
					vecty.Class("log-wrapper"),
				),
				// a.log,
				// &splitter.Splitter{
				// 	Selector:         ".log-wrapper",
				// 	OppositeSelector: ".content-wrapper",
				// 	Type:             splitter.BottomPane,
				// 	MinSizePercent:   2,
				// },
			),
		),

		// elem.Div(drawboard.New(a.actions)),
		vecty.If(a.isDrawingMode, a.DrawBoard),
		// drawboard.New(a.actions),

	)
}
