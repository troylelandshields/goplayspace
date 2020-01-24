package app

import (
	"encoding/json"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/iafan/goplayspace/client/api"
	"github.com/iafan/goplayspace/client/component/drawboard"
	"github.com/iafan/goplayspace/client/component/editor"
	"github.com/iafan/goplayspace/client/component/editor/undo"
	"github.com/iafan/goplayspace/client/component/log"
	"github.com/iafan/goplayspace/client/draw"
	"github.com/iafan/goplayspace/client/hash"
	"github.com/iafan/goplayspace/client/js/console"
	"github.com/iafan/goplayspace/client/js/window"
	"github.com/iafan/goplayspace/client/ranges"
	"github.com/iafan/goplayspace/client/util"
	"github.com/iafan/syntaxhighlight"
	"honnef.co/go/js/xhr"
)

const houseStr = `draw mode

// draw the roof
say Building the roof
color red
right 30
forward 5
right 120
forward 5
right 30

// draw the walls
say Building the walls
color black
forward 5
right
forward 5
right
forward 5
right
forward 5
right

// move to the door start
color off
forward 5
right
forward
right

// draw the door
say Building the door
color green
forward 2
left
forward
left
forward 2
left
forward

// move away from the house
color off
forward 3
left
say Done!`

const squaresStr = `draw mode
		
say Let's start...
right 18
color red

forward 7
say One...
right 144

forward 7
say Two...
right 144

forward 7
say Three...
right 144

forward 7
say Four...
right 144

forward 7
say We've got a star!
right 144`

const maxUndoStackSize uint = 50

const idDrawPage = "draw"

// Application implements the main application view
type Application struct {
	vecty.Core

	editor *editor.Editor
	log    *log.Log

	Input   string
	Topic   string
	Imports map[string]string

	// Settings
	Theme            string
	TabWidth         int
	FontWeight       string
	UseWebfont       bool
	HighlightingMode bool
	ShowSidebar      bool

	Hash      *hash.Hash
	snippetID string

	modifierKey          string
	isLoading            bool
	isCompiling          bool
	isSharing            bool
	isDrawingMode        bool
	hasCompilationErrors bool
	needRender           bool
	showSettings         bool
	showDrawHelp         bool

	// Log properties
	hasRun bool
	err    string
	events []*api.CompileEvent

	// Draw mode properties
	actions draw.ActorsList

	// Editor properties
	warningLines map[string]bool
	errorLines   map[string]bool
	undoStack    *undo.Stack
	changeTimer  *time.Timer
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
	a.isCompiling = true
	//a.doFormat()
	go a.doRunAsync()
}

func (a *Application) doRunAsync() {
	defer a.doRunAsyncComplete()

	a.hasRun = true

	bodyBytes, err := xhr.Send("POST", "/compile", []byte(a.Input))
	if err != nil {
		a.err = err.Error()
		return
	}

	compileResponse := api.CompileResponse{}

	err = json.Unmarshal(bodyBytes, &compileResponse)
	if err != nil {
		a.err = err.Error()
		return
	}

	a.err = compileResponse.Errors
	a.events = compileResponse.Events
	a.hasCompilationErrors = a.err != ""

	if compileResponse.Body != nil {
		a.setEditorText(*compileResponse.Body)
	}

	// extract line numbers from compilation error message

	if matches := compileErrorLineExtractorR.FindAllStringSubmatch(compileResponse.Errors, -1); matches != nil {
		a.errorLines = make(map[string]bool)
		for _, m := range matches {
			a.errorLines[m[1]] = true
		}
	}

	// parse gopher commands
	if !a.hasCompilationErrors {
		output := make([]string, len(a.events))
		for i := range a.events {
			output[i] = a.events[i].Message
		}
		a.actions = draw.New([]string{strings.Join(output, "\n"), houseStr, squaresStr})
		a.isDrawingMode = a.actions != nil
	}
}

func (a *Application) doRunAsyncComplete() {
	a.isCompiling = false
	a.wantRerender("doRunAsyncComplete")
	util.Schedule(func() { a.log.ScrollToBottom() })
}

func (a *Application) shareButtonClick(e *vecty.Event) {
	a.doShare()
}

func (a *Application) doShare() {
	a.isSharing = true
	a.doFormat()
	go a.doShareAsync()
}

func (a *Application) doShareAsync() {
	defer a.doShareAsyncComplete()

	bodyBytes, err := xhr.Send("POST", "/share", []byte(a.Input))
	if err != nil {
		a.err = err.Error()
		return
	}

	a.snippetID = string(bodyBytes) // already 'loaded'
	a.Hash.SetID(a.snippetID)
}

func (a *Application) doShareAsyncComplete() {
	a.isSharing = false
	a.wantRerender("doShareAsyncComplete")
}

func (a *Application) updateStateFromHash(h *hash.Hash) (canLoad bool) {
	if h.ID == idDrawPage {
		a.showDrawHelp = true
		return false
	}

	return true
}

func (a *Application) onHashChange(h *hash.Hash) {
	defer a.wantRerender("onHashChange")

	if a.isLoading || h.ID == "" {
		return
	}

	if a.updateStateFromHash(h) {
		a.doLoad(h.ID)
	}
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
		a.err = err.Error()
		return
	}
	if req.Status != 200 {
		a.err = req.ResponseText
		return
	}

	a.setEditorText(req.ResponseText)
	// setting new text will cause OnChange event,
	// and hash will be reset; so update it afterwards
	a.Hash.ID = id
}

func (a *Application) doLoadAsyncComplete(id string) {
	a.isLoading = false
	a.snippetID = id
	a.wantRerender("doLoadAsyncComplete")
}

func (a *Application) formatButtonClick(e *vecty.Event) {
	a.doFormat()
}

func (a *Application) format(text string) (string, error) {
	if a.Input == "" {
		return "", nil
	}

	//console.Time("format")
	bytes, err := format.Source([]byte(a.Input))
	//console.TimeEnd("format")

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (a *Application) doFormat() {
	defer util.Schedule(a.editor.Focus)
	a.wantRerender("doFormat")

	text, err := a.format(a.Input)
	if err != nil {
		a.err = err.Error()
	} else {
		a.err = ""
		a.setEditorText(text)
	}
}

func (a *Application) setEditorText(text string) {
	if a.Input == text {
		return
	}
	a.Input = text
	a.parseAndReportErrors(text)
	a.editor.SetText(text)
	util.Schedule(a.editor.Focus)
}

func (a *Application) setEditorState(text string, selStart, selEnd int) {
	if a.Input == text {
		return
	}
	a.Input = text
	a.parseAndReportErrors(text)
	a.editor.SetState(text, selStart, selEnd)
	util.Schedule(a.editor.Focus)
}

func (a *Application) onEditorValueChange(text string) {
	if a.Input == text {
		return
	}
	a.Input = text
	a.parseAndReportErrors(text)
	a.Hash.Reset()
	a.wantRerender("onEditorValueChange")
}

func (a *Application) parseAndReportErrors(text string) {
	a.err = ""
	a.warningLines = nil
	a.errorLines = nil
	a.hasCompilationErrors = false

	if text == "" {
		a.setEditorState(blankTemplate, blankTemplatePos, blankTemplatePos)
	}

	// parse source code to get list of imports and parsing error, if any;
	// note that we don't clear the list of imports since we want to
	// keep the previously known good mapping even if there are parsing errors

	fset := token.NewFileSet()
	//console.Time("parse")
	f, err := parser.ParseFile(fset, "", a.Input, parser.AllErrors)
	//console.TimeEnd("parse")

	a.Imports = make(map[string]string)
	if f != nil {
		for _, imp := range f.Imports {
			var name string
			path := strings.Trim(imp.Path.Value, `"`)
			if imp.Name != nil {
				name = imp.Name.Name
			} else {
				name = path
				if i := strings.LastIndex(path, "/"); i >= -1 {
					name = path[i+1:]
				}
			}

			// FIXME: should we somehow deal with '.' and '_' import names?

			if name != "." && name != "_" {
				a.Imports[name] = path // short package name
			}
			if path != "." && path != "_" && path != name {
				a.Imports[path] = path // full package name
			}
		}
	}

	if err != nil {
		a.err = err.Error()

		// extract line numbers from parser error message

		if matches := fmtErrorLineExtractorR.FindAllStringSubmatch(a.err, -1); matches != nil {
			a.warningLines = make(map[string]bool)
			for _, m := range matches {
				a.warningLines[m[1]] = true
			}
		}
	}
}

// highlight function is used to highlight source code in the editor
func (a *Application) highlight(text string) string {
	//console.Time("highlight")
	//defer console.TimeEnd("highlight")
	hbytes, err := syntaxhighlight.AsHTML([]byte(text), syntaxhighlight.OrderedList())
	if err != nil {
		console.Log("Highlight error:", err)
		a.err = err.Error()
		return ""
	}
	return string(hbytes)
}

func (a *Application) getGlobalState() (out string) {
	out = "ok"
	if a.err != "" {
		out = "warning"
		if a.hasCompilationErrors {
			out = "error"
		}
	}
	return
}

func (a *Application) getFiraFontCSS(weight, suffix string) string {
	return `@font-face {
	font-family: 'Fira Code';
	font-weight: ` + weight + `;
	src: url('https://raw.githubusercontent.com/tonsky/FiraCode/master/distr/woff2/FiraCode-` + suffix + `.woff2') format('woff2');
}`
}

func (a *Application) getOverrideCSS() (out string) {
	if a.UseWebfont {
		if a.FontWeight == "normal" {
			out += a.getFiraFontCSS(a.FontWeight, "Regular")
		} else {
			out += a.getFiraFontCSS(a.FontWeight, "Light")
		}
	}

	out += `.editor, .shadow, .log {
	font-weight: ` + a.FontWeight + `;
}`
	return out
}

// Mount implements the vecty.Mounter interface.
func (a *Application) Mount() {
	switch a.Hash.ID {
	case "":
		a.setEditorState(initialCode, initialCaretPos, initialCaretPos)
	case idDrawPage:
		a.setEditorState(initialDrawCode, initialDrawCaretPos, initialDrawCaretPos)
		fallthrough
	default:
		a.onHashChange(a.Hash)
	}
	window.AddEventListener("resize", a.onResize)
}

// Unmount implements the vecty.Unmounter interface.
func (a *Application) Unmount() {
	window.RemoveEventListener("resize", a.onResize)
}

func (a *Application) onResize() {
	a.editor.ResizeTextarea()
}

// Render renders the application
func (a *Application) Render() vecty.ComponentOrHTML {
	defer a.doRun()
	//console.Time("app:render")
	//defer console.TimeEnd("app:render")
	fmt.Println("Rendering app")

	if a.Hash == nil {
		a.Hash = hash.New(a.onHashChange)
		a.updateStateFromHash(a.Hash)
	}

	if a.undoStack == nil {
		a.undoStack = undo.NewStack(maxUndoStackSize)
	}

	if a.modifierKey == "" {
		a.modifierKey = "Ctrl"
		if util.IsMacOS() {
			a.modifierKey = "⌘"
		}
	}

	if a.editor == nil {
		a.editor = &editor.Editor{
			Highlighter: a.highlight,
			OnChange:    a.onEditorValueChange,
			// OnTopicChange: topicHandler,
			ChangeTimer: &a.changeTimer,
			UndoStack:   a.undoStack,
		}
	}
	a.editor.WarningLines = a.warningLines
	a.editor.ErrorLines = a.errorLines
	a.editor.Range = ranges.New(a.Hash.Ranges)
	a.editor.HighlightingMode = a.HighlightingMode
	a.editor.ReadonlyMode = a.isDrawingMode

	a.log = &log.Log{
		Error:  a.err,
		Events: a.events,
		HasRun: a.hasRun,
	}

	tabWidthClass := "tabwidth-" + strconv.Itoa(a.TabWidth)

	return elem.Body(
		vecty.Markup(
			vecty.Class(a.Theme),
			vecty.Class(tabWidthClass),
			vecty.Class(a.getGlobalState()),
			vecty.MarkupIf(util.IsSafari(), vecty.Class("safari")),
			vecty.MarkupIf(util.IsIOS(), vecty.Class("ios")),
			vecty.MarkupIf(a.isDrawingMode, vecty.Class("drawingmode")),
			vecty.MarkupIf(a.ShowSidebar, vecty.Class("withsidebar")),
		),
		elem.Div(
			vecty.Markup(
				vecty.Class("header"),
			),
		),
		elem.Div(
			vecty.Markup(
				vecty.Class("body-wrapper"),
			),
		),
		// elem.Div(drawboard.New(a.actions)),
		vecty.If(a.isDrawingMode, drawboard.New(a.actions)),
		// drawboard.New(a.actions),
		elem.Style(
			vecty.Markup(
				vecty.UnsafeHTML(a.getOverrideCSS()),
			),
		),
	)
}
