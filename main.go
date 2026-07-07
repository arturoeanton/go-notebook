//go:debug gotypesalias=0

package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/arturoeanton/go-notebook/bindings"
	"github.com/cosmos72/gomacro/base"
	"github.com/cosmos72/gomacro/base/inspect"
	"github.com/cosmos72/gomacro/fast"
	"github.com/cosmos72/gomacro/fast/debug"
	"github.com/cosmos72/gomacro/imports"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func init() {
	// Make the bundled libraries (go-pandas, go-dsl, go-xml) importable from
	// notebook cells without network access or plugin compilation.
	imports.Packages.Merge(bindings.Packages)
}

// LinkMenu is a toolbar action button.
type LinkMenu struct {
	Driver  *liveview.ComponentDriver[*LinkMenu] `json:"-"`
	Caption string                               `json:"caption"`
	Class   string                               `json:"class"`
	Title   string                               `json:"title"`
	Id      string                               `json:"id"`
}

func (t *LinkMenu) GetDriver() liveview.LiveDriver {
	return t.Driver
}

func (t *LinkMenu) Start() {
	t.Driver.Commit()
}

func (t *LinkMenu) GetTemplate() string {
	return `<button type="button" class="btn {{.Class}}" title="{{.Title}}" onclick="send_event('{{.Id}}','Click')">{{.Caption}}</button>`
}

func newInterpreter() *fast.Interp {
	interp := fast.New()
	interp.SetDebugger(&debug.Debugger{})
	interp.SetInspector(&inspect.Inspector{})

	g := &interp.Comp.Globals
	g.ParserMode = 0
	g.Options |= base.OptDebugger |
		base.OptCtrlCEnterDebugger |
		base.OptTrapPanic |
		base.OptModuleImport
	g.Options &^= base.OptShowPrompt | base.OptShowEval | base.OptShowEvalType

	g.Imports, g.Declarations, g.Statements = nil, nil, nil

	defaultCode, _ := liveview.FileToString("default.gonote")
	interp.Eval(defaultCode)
	return interp
}

func main() {
	filename := flag.String("f", "notebooks/default.gonote.json", "GoNotebookFile")
	port := flag.String("p", ":1323", "port")
	flag.Parse()
	notebookFile = *filename

	e := echo.New()
	e.HideBanner = true
	e.Static("/static", "static")
	css, _ := liveview.FileToString("lives/style.css")
	home := liveview.PageControl{
		Title:    "Go Notebook",
		Lang:     "en",
		HeadCode: "lives/head.html",
		Css:      css,
		Path:     "/",
		Router:   e,
	}

	// Warm up the interpreter machinery once at startup.
	newInterpreter()

	home.Register(func() liveview.LiveDriver {
		interp := newInterpreter()

		page := liveview.NewLayout("notebook", "lives/layout.html")

		linkAddCode := liveview.NewDriver("link_add_code", &LinkMenu{
			Caption: `<span class="btn-ico">+</span> Code`,
			Title:   "Add a Go code cell",
		})
		linkAddCode.Events["Click"] = func(c *LinkMenu, data interface{}) {
			idShell := "shell_" + uuid.NewString()
			code := ""

			if snippetName, ok := data.(string); ok && snippetName != "" {
				if liveview.Exists("snippet/" + snippetName + ".gonote") {
					code, _ = liveview.FileToString("snippet/" + snippetName + ".gonote")
				}
			}

			newShell := &Shell{Debug: true, Interpreter: interp, CaptionPlay: "Play", Code: code}
			addShell(idShell, page, newShell)
		}

		linkAddMD := liveview.NewDriver("link_add_md", &LinkMenu{
			Caption: `<span class="btn-ico">+</span> Text`,
			Title:   "Add a markdown cell",
		})
		linkAddMD.Events["Click"] = func(c *LinkMenu, data interface{}) {
			idText := "text_" + uuid.NewString()
			addMD(idText, page, &TextMD{CaptionPlay: "Print"})
		}

		linkReload := liveview.NewDriver("link_reload", &LinkMenu{
			Caption: `<span class="btn-ico">⟳</span> Reload`,
			Title:   "Reload the current notebook",
		})
		reloadNotebook := func(data interface{}) {
			note = Note{Order: make([]string, 0), Items: make(map[string]Item)}
			linkReload.FillValueById("main_div", "")
			defaultCode, _ := liveview.FileToString("default.gonote")
			interp.Eval(defaultCode)
			if name, ok := data.(string); ok && name != "" {
				notebookFile = "notebooks/" + name + ".gonote.json"
			}
			if !liveview.Exists(notebookFile) {
				liveview.StringToFile(notebookFile, `{"order":[], "Items":{}}`)
			}
			jsonString, _ := liveview.FileToString(notebookFile)
			json.Unmarshal([]byte(jsonString), &note)
			for _, key := range note.Order {
				item := note.Items[key]
				if item.MD != nil {
					addMD(key, page, item.MD)
				}
				if item.Shell != nil {
					item.Shell.Interpreter = interp
					item.Shell.CaptionPlay = "Play"
					addShell(key, page, item.Shell)
				}
			}

			options := "<option value=\"\">Insert snippet…</option>"
			filepath.Walk("snippet/", func(path string, info os.FileInfo, err error) error {
				if err == nil && strings.HasSuffix(info.Name(), ".gonote") {
					name := strings.Split(info.Name(), ".")[0]
					options += "<option>" + name + "</option>"
				}
				return nil
			})
			linkReload.FillValueById("snippet", options)

			optionsNotebook := ""
			filepath.Walk("notebooks/", func(path string, info os.FileInfo, err error) error {
				if err == nil && strings.HasSuffix(info.Name(), ".gonote.json") {
					name := strings.TrimSuffix(info.Name(), ".gonote.json")
					if name != "" {
						optionsNotebook += "<option>" + name + "</option>"
					}
				}
				return nil
			})
			linkReload.FillValueById("notebooks", optionsNotebook)

			notebookName := strings.TrimSuffix(filepath.Base(notebookFile), ".gonote.json")
			linkReload.FillValueById("notebook_name", notebookName)
			linkReload.EvalScript("var nb=document.getElementById('notebook'); if(nb){nb.value=" + jsString(notebookName) + "}")
		}
		linkReload.Events["Click"] = func(c *LinkMenu, data interface{}) {
			reloadNotebook(data)
		}

		// Load the notebook as soon as the layout is mounted: triggering this
		// server side avoids racing the WebSocket handshake from JavaScript.
		page.Component.SetHandlerFirstTime(func() {
			reloadNotebook(nil)
		})

		return page.Mount(linkAddCode.Component).
			Mount(linkAddMD.Component).
			Mount(linkReload.Component)
	})

	e.Logger.Fatal(e.Start(*port))
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func addShell(idShell string, page *liveview.ComponentDriver[*liveview.Layout], newShell *Shell) {
	page.AddNode("main_div", `<div id="`+idShell+`" class="cell-slot"></div>`)
	shell := liveview.NewDriver(idShell, newShell)
	page.MountWithStart(idShell, shell)
	scrollToCell(page, idShell)
}

func addMD(idText string, page *liveview.ComponentDriver[*liveview.Layout], newTextMD *TextMD) {
	page.AddNode("main_div", `<div id="`+idText+`" class="cell-slot"></div>`)
	textMD := liveview.NewDriver(idText, newTextMD)
	page.MountWithStart(idText, textMD)
	scrollToCell(page, idText)
}

func scrollToCell(page *liveview.ComponentDriver[*liveview.Layout], id string) {
	page.EvalScript("var el=document.getElementById(" + jsString(id) + "); if(el){el.scrollIntoView({behavior:'smooth',block:'end'})}")
}
