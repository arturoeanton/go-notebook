//go:debug gotypesalias=0

package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
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
	filename := flag.String("f", "default", "default notebook name")
	port := flag.String("p", ":1323", "port")
	flag.Parse()
	defaultNotebook := sanitizeNotebookName(*filename)
	if defaultNotebook == "" {
		defaultNotebook = "default"
	}

	e := echo.New()
	e.HideBanner = true
	e.Static("/static", "static")
	e.GET("/export/:name", exportHandler)
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
		session := newSession(defaultNotebook, interp)
		declareHostFuncs(interp, session)

		page := liveview.NewLayout("notebook", "lives/layout.html")
		session.page = page

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

			newShell := &Shell{Debug: true, Session: session, Interpreter: interp, CaptionPlay: "Play", Code: code}
			addShell(idShell, page, newShell)
		}

		linkAddMD := liveview.NewDriver("link_add_md", &LinkMenu{
			Caption: `<span class="btn-ico">+</span> Text`,
			Title:   "Add a markdown cell",
		})
		linkAddMD.Events["Click"] = func(c *LinkMenu, data interface{}) {
			idText := "text_" + uuid.NewString()
			addMD(idText, page, &TextMD{Session: session, CaptionPlay: "Print"})
		}

		linkReload := liveview.NewDriver("link_reload", &LinkMenu{
			Caption: `<span class="btn-ico">⟳</span> Reload`,
			Title:   "Reload the current notebook",
		})

		refreshSidebar := func() {
			current := session.notebookName()
			var sb strings.Builder
			for _, n := range listNotebooks() {
				active := ""
				if n == current {
					active = " nb-active"
				}
				esc := "'" + n + "'" // names are sanitized: no quotes possible
				sb.WriteString(`<li class="nb-item` + active + `">`)
				sb.WriteString(`<a class="nb-name" onclick="send_event('link_reload','Open',` + esc + `)">` + n + `</a>`)
				sb.WriteString(`<span class="nb-actions">`)
				sb.WriteString(`<button title="Rename" onclick="nbRename(` + esc + `)">✎</button>`)
				sb.WriteString(`<button title="Duplicate" onclick="send_event('link_reload','Duplicate',` + esc + `)">⧉</button>`)
				sb.WriteString(`<button title="Delete" onclick="nbDelete(` + esc + `)">✕</button>`)
				sb.WriteString(`</span></li>`)
			}
			linkReload.FillValueById("nb_list", sb.String())
		}

		openNotebook := func(name string) {
			name = sanitizeNotebookName(name)
			if name == "" {
				name = defaultNotebook
			}
			linkReload.FillValueById("main_div", "")
			defaultCode, _ := liveview.FileToString("default.gonote")
			interp.Eval(defaultCode)
			session.load(name)
			for _, key := range session.snapshotOrder() {
				item := session.item(key)
				if item.MD != nil {
					item.MD.Session = session
					addMD(key, page, item.MD)
				}
				if item.Shell != nil {
					item.Shell.Session = session
					item.Shell.Interpreter = interp
					item.Shell.CaptionPlay = "Play"
					addShell(key, page, item.Shell)
				}
			}

			options := "<option value=\"\">Insert snippet…</option>"
			filepath.Walk("snippet/", func(path string, info os.FileInfo, err error) error {
				if err == nil && strings.HasSuffix(info.Name(), ".gonote") {
					n := strings.Split(info.Name(), ".")[0]
					options += "<option>" + n + "</option>"
				}
				return nil
			})
			linkReload.FillValueById("snippet", options)

			linkReload.FillValueById("notebook_name", name)
			refreshSidebar()
			// name is sanitized: safe to embed
			linkReload.EvalScript("localStorage.setItem('gonotebook-nb', '" + name + "'); window.scrollTo({top:0})")
		}

		linkReload.Events["Click"] = func(c *LinkMenu, data interface{}) {
			openNotebook(session.notebookName())
		}
		linkReload.Events["Open"] = func(c *LinkMenu, data interface{}) {
			if name, ok := data.(string); ok {
				openNotebook(name)
			}
		}
		linkReload.Events["Create"] = func(c *LinkMenu, data interface{}) {
			name := sanitizeNotebookName(toString(data))
			if name == "" {
				return
			}
			openNotebook(name) // load() creates the file when missing
		}
		linkReload.Events["Delete"] = func(c *LinkMenu, data interface{}) {
			name := sanitizeNotebookName(toString(data))
			if name == "" {
				return
			}
			os.Remove(notebookPath(name))
			if name == session.notebookName() {
				openNotebook(defaultNotebook)
				return
			}
			refreshSidebar()
		}
		linkReload.Events["Duplicate"] = func(c *LinkMenu, data interface{}) {
			name := sanitizeNotebookName(toString(data))
			if name == "" {
				return
			}
			src, err := os.ReadFile(notebookPath(name))
			if err != nil {
				return
			}
			copyName := name + "-copy"
			for i := 2; liveview.Exists(notebookPath(copyName)); i++ {
				copyName = name + "-copy" + strconv.Itoa(i)
			}
			liveview.StringToFile(notebookPath(copyName), string(src))
			refreshSidebar()
		}
		linkReload.Events["Rename"] = func(c *LinkMenu, data interface{}) {
			var req struct {
				Old string `json:"old"`
				New string `json:"new"`
			}
			if err := json.Unmarshal([]byte(toString(data)), &req); err != nil {
				return
			}
			oldName := sanitizeNotebookName(req.Old)
			newName := sanitizeNotebookName(req.New)
			if oldName == "" || newName == "" || oldName == newName {
				return
			}
			if liveview.Exists(notebookPath(newName)) {
				return
			}
			if err := os.Rename(notebookPath(oldName), notebookPath(newName)); err != nil {
				return
			}
			if oldName == session.notebookName() {
				openNotebook(newName)
				return
			}
			refreshSidebar()
		}

		// Load the notebook as soon as the layout is mounted: triggering this
		// server side avoids racing the WebSocket handshake from JavaScript.
		page.Component.SetHandlerFirstTime(func() {
			openNotebook(session.notebookName())
		})
		// Persist any pending changes when the connection closes.
		page.Component.SetHandlerEventDestroy(func(id string) {
			session.Flush()
		})

		return page.Mount(linkAddCode.Component).
			Mount(linkAddMD.Component).
			Mount(linkReload.Component)
	})

	e.Logger.Fatal(e.Start(*port))
}

func toString(data interface{}) string {
	s, _ := data.(string)
	return s
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

func newGridDriver(g *DataGrid) liveview.LiveDriver {
	return liveview.NewDriver(g.Id, g)
}
