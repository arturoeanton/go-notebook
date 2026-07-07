package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"log"
	"os"
	"strings"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/cosmos72/gomacro/fast"
	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
)

// TextMD is a markdown cell. It toggles between an editor and the rendered
// preview depending on whether Result is empty.
type TextMD struct {
	Driver      *liveview.ComponentDriver[*TextMD] `json:"-"`
	UUID        string                             `json:"uuid"`
	Id          string                             `json:"id"`
	CodeMD      string                             `json:"code_md"`
	Result      string                             `json:"result"`
	CaptionPlay string                             `json:"caption_play"`
}

func (t *TextMD) GetDriver() liveview.LiveDriver {
	return t.Driver
}

func (t *TextMD) GetTemplate() string {
	te, _ := liveview.FileToString("lives/text_md.html")
	return te
}

func (t *TextMD) Start() {
	t.UUID = uuid.New().String()
	t.Driver.Commit()
	if !liveview.ContainsString(note.Order, t.Id) {
		note.Order = append(note.Order, t.Id)
	}
	note.Items[t.Id] = Item{MD: t}
	saveNote()
}

func (t *TextMD) Click(data interface{}) {
	if t.Result != "" {
		t.Result = ""
		t.Driver.Commit()
		note.Items[t.Id] = Item{MD: t}
		saveNote()
		return
	}
	t.CodeMD = t.Driver.GetElementById(t.UUID + "_text_md")
	md := []byte(t.CodeMD)
	t.Result = string(markdown.ToHTML(md, nil, nil))
	t.Driver.Commit()
	note.Items[t.Id] = Item{MD: t}
	saveNote()
}

func (t *TextMD) Remove(data interface{}) {
	delete(note.Items, t.Id)
	for i, id := range note.Order {
		if id == t.Id {
			note.Order = append(note.Order[:i], note.Order[i+1:]...)
			break
		}
	}
	t.Driver.Remove(t.Id)
	saveNote()
}

// Shell is a Go code cell evaluated with gomacro.
type Shell struct {
	Driver      *liveview.ComponentDriver[*Shell] `json:"-"`
	Interpreter *fast.Interp                      `json:"-"`
	Debug       bool                              `json:"debug"`
	UUID        string                            `json:"uuid"`
	Id          string                            `json:"id"`
	Code        string                            `json:"code"`
	CaptionPlay string                            `json:"caption_play"`
	Result      string                            `json:"-"`
	Out         string                            `json:"-"`
	Err         string                            `json:"-"`
}

func (t *Shell) GetDriver() liveview.LiveDriver {
	return t.Driver
}

func (t *Shell) GetTemplate() string {
	te, _ := liveview.FileToString("lives/shell.html")
	return te
}

func (t *Shell) Start() {
	t.Out = ""
	t.UUID = uuid.New().String()
	t.Driver.Commit()
	if !liveview.ContainsString(note.Order, t.Id) {
		note.Order = append(note.Order, t.Id)
	}
	note.Items[t.Id] = Item{Shell: t}
	saveNote()
}

func (t *Shell) Remove(data interface{}) {
	delete(note.Items, t.Id)
	for i, id := range note.Order {
		if id == t.Id {
			note.Order = append(note.Order[:i], note.Order[i+1:]...)
			break
		}
	}
	t.Driver.Remove(t.Id)
	saveNote()
}

func (t *Shell) Click(data interface{}) {
	t.Driver.EvalScript("var c=document.getElementById('" + t.UUID + "_div_code'); if(c){c.classList.add('cell--running')}")

	old := os.Stdout // keep backup of the real stdout
	oldErr := os.Stderr
	r, w, _ := os.Pipe()

	os.Stderr = w
	os.Stdout = w
	log.SetOutput(w)

	t.Code = t.Driver.GetElementById(t.UUID + "_code")
	t.Result = ""
	t.Err = ""
	defer func() {
		if rec := recover(); rec != nil {
			t.Err = html.EscapeString(fmt.Sprint(rec))
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
		t.Out = html.EscapeString(<-outC)

		t.Driver.Commit()
		note.Items[t.Id] = Item{Shell: t}
		saveNote()
	}()

	values, types := t.Interpreter.Eval(t.Code)
	for i, value := range values {
		v := fmt.Sprint(value.ReflectValue())
		if strings.HasPrefix(strings.TrimSpace(v), "<div") {
			t.Result += v
			continue
		}
		t.Result += "<div class=\"res-item\"><pre class=\"res-value\">" +
			html.EscapeString(v) + "</pre><span class=\"res-type\">" +
			html.EscapeString(fmt.Sprint(types[i])) + "</span></div>"
	}
}
