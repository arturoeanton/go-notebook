package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/arturoeanton/go-echo-live-view/components"
	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/cosmos72/gomacro/fast"
	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type TextMD struct {
	Driver *liveview.ComponentDriver
	UUID   string
	Id     string
	CodeMD string
	Result string
}

func (t *TextMD) GetTemplate() string {
	return `<hr/>
	<fieldset>
	<legend>Text</legend>
	{{if eq .Result "" }} 
	<textarea  id="{{.UUID}}_text_md" style="width: 100%;" >{{.CodeMD}}</textarea>
	{{end }} 
	{{if ne .Result "" }} 
	<fieldset>
	{{.Result}}
	</fieldset>
	{{end}}
	<hr/>
	<button id="{{.UUID}}_play" onclick="send_event('{{.Id}}', 'Click')" >Convert</button>
	</fieldset>

	`
}

func (t *TextMD) Start() {
	t.UUID = uuid.New().String()
	t.Driver.Commit()
}

func (t *TextMD) Click(data interface{}) {
	if t.Result != "" {
		t.Result = ""
		t.Driver.Commit()
		return
	}
	t.CodeMD = t.Driver.GetValue(t.UUID + "_text_md")
	md := []byte(t.CodeMD)
	t.Result = string(markdown.ToHTML(md, nil, nil))
	t.Driver.Commit()
}

type Shell struct {
	Driver      *liveview.ComponentDriver
	Interpreter *fast.Interp
	Debug       bool
	UUID        string
	Id          string
	Code        string
	CaptionPlay string
	Result      string
	Out         string
	Err         string
}

func (t *Shell) Start() {
	t.CaptionPlay = "Play"
	t.Out = ""
	t.UUID = uuid.New().String()
	t.Driver.Commit()
}

func (t *Shell) GetTemplate() string {
	return `<hr/>
	<fieldset>
	<legend>Code</legend>
	<textarea  id="{{.UUID}}_code" style="width: 100%;" >{{.Code}}</textarea>
	{{if ne .Result "" }} 
	<fieldset>
	<legend>Result</legend>
	{{.Result}}
	</fieldset>
	{{end}}
	{{if ne .Out "" }} 
	<fieldset>
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
	</fieldset>`
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
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	home := liveview.PageControl{
		Title:  "Go-Notebook",
		Lang:   "en",
		Path:   "/",
		Router: e,
	}

	home.Register(func() *liveview.ComponentDriver {
		interp := fast.New()
		page := components.NewLayout("notebook", `{{ mount "button_add_code"}} {{mount "button_add_md"}}`)
		button_add_code := liveview.NewDriver("button_add_code", &components.Button{Caption: "Add Code"})
		button_add_code.Events["Click"] = func(data interface{}) {
			button := button_add_code.Component.(*components.Button)
			button.I++
			content := button_add_code.GetHTML("content")
			content += `<div id="shell` + fmt.Sprint(button.I) + `"><div>`
			button_add_code.SetHTML("content", content)
			idShell := "shell" + fmt.Sprint(button.I)
			shell := liveview.NewDriver(idShell, &Shell{Debug: true, Interpreter: interp})
			page.MountWithStart(idShell, shell)
		}
		button_add_md := liveview.NewDriver("button_add_md", &components.Button{Caption: "Add Text"})
		button_add_md.Events["Click"] = func(data interface{}) {
			button := button_add_md.Component.(*components.Button)
			button.I++
			content := button_add_md.GetHTML("content")
			content += `<div id="text` + fmt.Sprint(button.I) + `"><div>`
			button_add_md.SetHTML("content", content)
			idText := "text" + fmt.Sprint(button.I)
			textMD := liveview.NewDriver(idText, &TextMD{})
			page.MountWithStart(idText, textMD)
		}
		return page.Mount(button_add_code).Mount(button_add_md)

	})

	e.Logger.Fatal(e.Start(":1323"))
}
