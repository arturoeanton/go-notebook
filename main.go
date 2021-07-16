package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arturoeanton/go-echo-live-view/components"
	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/arturoeanton/gocommons/utils"
	"github.com/cosmos72/gomacro/base"
	"github.com/cosmos72/gomacro/base/inspect"
	"github.com/cosmos72/gomacro/fast"
	"github.com/cosmos72/gomacro/fast/debug"
	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

var (
	note         Note = Note{Order: make([]string, 0), Items: make(map[string]Item)}
	notebookFile      = "data.gonote.json"
)

func MapToJSONFile(file string, data interface{}) {
	jsonString, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}
	utils.StringToFile(file, string(jsonString))
}

type Note struct {
	Order []string `json:"order"`
	Items map[string]Item
}

type Item struct {
	MD    *TextMD `json:"md,omitempty"`
	Shell *Shell  `json:"shell,omitempty"`
}

type LinkMenu struct {
	Driver  *liveview.ComponentDriver `json:"-"`
	Caption string                    `json:"caption"`
	Id      string                    `json:"id"`
}

func (t *LinkMenu) Start() {
	t.Driver.Commit()
}

func (t *LinkMenu) GetTemplate() string {
	return `<a  href="#" onClick="send_event('{{.Id}}','Click')" >{{.Caption}}</a>`
}

type TextMD struct {
	Driver      *liveview.ComponentDriver `json:"-"`
	UUID        string                    `json:"uuid"`
	Id          string                    `json:"id"`
	CodeMD      string                    `json:"code_md"`
	Result      string                    `json:"result"`
	CaptionPlay string                    `json:"caption_play"`
}

func (t *TextMD) GetTemplate() string {
	te, _ := utils.FileToString(`lives/text_md.html`)
	return te
}

func (t *TextMD) Start() {
	t.UUID = uuid.New().String()
	t.Driver.Commit()
	if !utils.ContainsString(note.Order, t.Id) {
		note.Order = append(note.Order, t.Id)
	}

	note.Items[t.Id] = Item{MD: t}
	MapToJSONFile(notebookFile, note)
}

func (t *TextMD) Click(data interface{}) {
	if t.Result != "" {
		t.Result = ""
		t.CaptionPlay = "Print"
		t.Driver.Commit()
		note.Items[t.Id] = Item{MD: t}
		MapToJSONFile(notebookFile, note)
		return
	}
	t.CaptionPlay = "Edit"
	t.CodeMD = t.Driver.GetValue(t.UUID + "_text_md")
	md := []byte(t.CodeMD)
	t.Result = string(markdown.ToHTML(md, nil, nil))
	t.Driver.Commit()
	note.Items[t.Id] = Item{MD: t}
	MapToJSONFile(notebookFile, note)
}

func (t *TextMD) Remove(data interface{}) {
	delete(note.Items, t.Id)
	t.Driver.Remove(t.UUID + "_div_md")
	MapToJSONFile(notebookFile, note)
}

type Shell struct {
	Driver      *liveview.ComponentDriver `json:"-"`
	Interpreter *fast.Interp              `json:"-"`
	Debug       bool                      `json:"debug"`
	UUID        string                    `json:"uuid"`
	Id          string                    `json:"id"`
	Code        string                    `json:"code"`
	CaptionPlay string                    `json:"caption_play"`
	Result      string                    `json:"-"`
	Out         string                    `json:"-"`
	Err         string                    `json:"-"`
}

func (t *Shell) Start() {
	t.Out = ""
	t.UUID = uuid.New().String()
	t.Driver.Commit()
	if !utils.ContainsString(note.Order, t.Id) {
		note.Order = append(note.Order, t.Id)
	}
	note.Items[t.Id] = Item{Shell: t}
	MapToJSONFile(notebookFile, note)
}

func (t *Shell) GetTemplate() string {
	te, _ := utils.FileToString("lives/shell.html")
	return te
}

func (t *Shell) Remove(data interface{}) {
	delete(note.Items, t.Id)
	t.Driver.Remove(t.UUID + "_div_code")
	MapToJSONFile(notebookFile, note)
}
func (t *Shell) Click(data interface{}) {
	old := os.Stdout // keep backup of the real stdout
	oldErr := os.Stderr
	r, w, _ := os.Pipe()

	os.Stderr = w
	os.Stdout = w
	log.SetOutput(w)

	t.Code = t.Driver.GetValue(t.UUID + "_code")
	t.Result = ""
	defer func() {
		if r := recover(); r != nil {
			t.Result += "Recovered in (" + t.Code + ")" + fmt.Sprint(r)
			t.CaptionPlay = "(E)Play"
		}

		outC := make(chan string)
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r)
			outC <- buf.String()
		}()
		w.Close()
		os.Stdout = old
		os.Stderr = oldErr
		log.SetOutput(os.Stderr)
		t.Out = <-outC

		t.Driver.Commit()
		note.Items[t.Id] = Item{Shell: t}
		MapToJSONFile(notebookFile, note)
	}()

	values, types := t.Interpreter.Eval(t.Code)
	for i, value := range values {
		t.Result += fmt.Sprint(value.ReflectValue())
		if t.Debug {
			t.Result += "|" + fmt.Sprint(types[i])
		}
		t.Result += "\n"
	}
	t.CaptionPlay = "Replay"

}

func main() {

	filename := flag.String("f", "default.gonote.json", "GoNotebookFile")
	port := flag.String("p", ":1323", "port")
	flag.Parse()
	notebookFile = *filename

	e := echo.New()
	e.Static("/static", "static")
	css, _ := utils.FileToString("lives/style.css")
	home := liveview.PageControl{
		Title:     "Go-Notebook",
		Lang:      "en",
		HeadCode:  "lives/head.html",
		AfterCode: "lives/after.html",
		Css:       css,
		Path:      "/",
		Router:    e,
	}

	interpSetup := fast.New()
	defaultCode, _ := utils.FileToString("default.gonote")
	interpSetup.Eval(defaultCode)

	home.Register(func() *liveview.ComponentDriver {
		interp := fast.New()
		interp.SetDebugger(&debug.Debugger{})
		interp.SetInspector(&inspect.Inspector{})

		g := &interp.Comp.Globals
		g.ParserMode = 0 // defaults
		g.Options |= base.OptDebugger |
			base.OptCtrlCEnterDebugger |
			base.OptKeepUntyped |
			base.OptTrapPanic |
			base.OptShowPrompt |
			base.OptShowEval |
			base.OptShowEvalType |
			base.OptModuleImport |
			base.OptShowCompile |
			base.OptShowTime

		g.Options &^= base.OptShowPrompt | base.OptShowEval | base.OptShowEvalType // cleared by default, overridden by -s, -v and -vv

		g.Imports, g.Declarations, g.Statements = nil, nil, nil

		defaultCode, _ := utils.FileToString("default.gonote")
		interp.Eval(defaultCode)

		page := components.NewLayout("notebook", "lives/layout.html")

		link_add_code := liveview.NewDriver("link_add_code", &LinkMenu{Caption: "+ Code"})
		link_add_code.Events["Click"] = func(data interface{}) {
			idShell := "shell_" + uuid.NewString()
			code := ""

			if snippetName, ok := data.(string); ok {
				if utils.Exists("snippet/" + snippetName + ".gonote") {
					code, _ = utils.FileToString("snippet/" + snippetName + ".gonote")
				}
			}

			newShell := &Shell{Debug: true, Interpreter: interp, CaptionPlay: "Play", Code: code}
			addShell(idShell, page, link_add_code, newShell)
		}
		link_add_md := liveview.NewDriver("link_add_md", &LinkMenu{Caption: "+ Text"})
		link_add_md.Events["Click"] = func(data interface{}) {
			idText := "text_" + uuid.NewString()
			addMD(idText, page, link_add_md, &TextMD{CaptionPlay: "Print"})
		}

		link_reload := liveview.NewDriver("link_reload", &LinkMenu{Caption: "Reload"})
		link_reload.Events["Click"] = func(data interface{}) {
			jsonString, _ := utils.FileToString(notebookFile)
			json.Unmarshal([]byte(jsonString), &note)
			for _, key := range note.Order {
				item := note.Items[key]
				if item.MD != nil {
					addMD(key, page, link_reload, item.MD)
				}
				if item.Shell != nil {
					item.Shell.Interpreter = interp
					item.Shell.CaptionPlay = "Play"
					addShell(key, page, link_reload, item.Shell)
				}
			}

			options := "<option></option>"

			root := "snippet/"
			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if strings.HasSuffix(info.Name(), ".gonote") {
					options += "<option>" + strings.Split(info.Name(), ".")[0] + "</option>"
				}
				link_reload.SetHTML("snippet", options)
				return nil
			})

		}

		return page.Mount(link_add_code).Mount(link_add_md).Mount(link_reload)
	})

	e.Logger.Fatal(e.Start(*port))
}

func addShell(idShell string, page, driver *liveview.ComponentDriver, newShell *Shell) {
	main_div := driver.GetHTML("main_div")
	main_div += `<div id="` + idShell + `"><div>`
	driver.SetHTML("main_div", main_div)
	shell := liveview.NewDriver(idShell, newShell)
	page.MountWithStart(idShell, shell)
}

func addMD(idText string, page, driver *liveview.ComponentDriver, newTextMD *TextMD) {
	main_div := driver.GetHTML("main_div")
	main_div += `<div id="` + idText + `"><div>`
	driver.SetHTML("main_div", main_div)
	textMD := liveview.NewDriver(idText, newTextMD)
	page.MountWithStart(idText, textMD)
}
