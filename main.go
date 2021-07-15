package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/arturoeanton/go-echo-live-view/components"
	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/arturoeanton/gocommons/utils"
	"github.com/cosmos72/gomacro/fast"
	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	return `
	<br>
	<div id="{{.UUID}}_div_md" style="background: white;">
		<fieldset style="border-radius: 5px">
			{{if eq .Result "" }} 
			<textarea  oninput="autosize(this)" onfocus="autosize(this)" id="{{.UUID}}_text_md" style="width: 100%; height: 50px; padding: 5px; background: cornsilk;" >{{.CodeMD}}</textarea>
			{{end }} 
			{{if ne .Result "" }} 
			<fieldset style="border-width: 0px;">
			{{.Result}}
			</fieldset>
			{{end}}
			<hr/>
			<button id="{{.UUID}}_play" onclick="send_event('{{.Id}}', 'Click')" >{{.CaptionPlay}}</button>
			<button id="{{.UUID}}_remove" onclick="send_event('{{.Id}}', 'Remove', '{{.UUID}}')" >Remove</button>
		</fieldset>
	</div>
	`
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
	return `
	<br>
	<div id="{{.UUID}}_div_code" style="background: white;">
		<fieldset style="border-radius: 5px">
			<textarea  oninput="autosize(this)" onfocus="autosize(this)" id="{{.UUID}}_code" style="width: 100%; height: 50px; padding: 5px; background: darkseagreen;" >{{.Code}}</textarea>
			{{if ne .Result "" }} 
			<fieldset style="background: aliceblue;">
				<legend>Result</legend>
				{{.Result}}
			</fieldset>
			{{end}}
			{{if ne .Out "" }} 
			<fieldset style="background: aliceblue;">
				<legend>Output</legend>
				{{.Out}} 
			</fieldset>
			{{end}}
			{{if ne .Err "" }} 
			<fieldset>
				<legend>Stderr</legend>
				{{.Err}} 
			</fieldset>
			{{end}}
			<hr/>
			<button id="{{.UUID}}_play" onclick="send_event('{{.Id}}', 'Click')" >{{.CaptionPlay}}</button>
			<button id="{{.UUID}}_remove" onclick="send_event('{{.Id}}', 'Remove', '{{.UUID}}')" >Remove</button>
		</fieldset>
	</div>`
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
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	home := liveview.PageControl{
		Title: "Go-Notebook",
		Lang:  "en",
		HeadCode: `
		<script src="https://rawgit.com/jackmoore/autosize/master/dist/autosize.min.js"></script>
		`,
		Css: `
		body {margin:0;}

.navbar {
  overflow: hidden;
  background-color: #333;
  position: fixed;
  top: 0;
  width: 100%;

}

.navbar a {
  float: left;
  display: block;
  color: #f2f2f2;
  text-align: center;
  padding: 14px 16px;
  text-decoration: none;
  font-size: 17px;
}

.navbar a:hover {
  background: #ddd;
  color: black;
}

#main_div {
	padding: 16px;
	margin-top: 30px;
  }
		`,
		Path:   "/",
		Router: e,
	}

	home.Register(func() *liveview.ComponentDriver {
		interp := fast.New()
		page := components.NewLayout("notebook", `
		<div class="navbar">
		{{ mount "link_add_code"}} 
		{{mount "link_add_md"}}
		{{mount "link_reload"}}
		</div>  
		<div id="main_div">
		</div>
		`)

		link_add_code := liveview.NewDriver("link_add_code", &LinkMenu{Caption: "+ Code"})
		link_add_code.Events["Click"] = func(data interface{}) {
			idShell := "shell_" + uuid.NewString()
			newShell := &Shell{Debug: true, Interpreter: interp, CaptionPlay: "Play"}
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
