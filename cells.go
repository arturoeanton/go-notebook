package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/cosmos72/gomacro/fast"
	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
)

// TextMD is a markdown cell. It toggles between an editor and the rendered
// preview depending on whether Result is empty.
type TextMD struct {
	Driver      *liveview.ComponentDriver[*TextMD] `json:"-"`
	Session     *Session                           `json:"-"`
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
	t.Session.touch(t.Id, Item{MD: t})
}

func (t *TextMD) Click(data interface{}) {
	if t.Result != "" {
		t.Result = ""
		t.Driver.Commit()
		t.Session.touch(t.Id, Item{MD: t})
		return
	}
	t.CodeMD = t.Driver.GetElementById(t.UUID + "_text_md")
	md := []byte(t.CodeMD)
	t.Result = string(markdown.ToHTML(md, nil, nil))
	t.Driver.Commit()
	t.Session.touch(t.Id, Item{MD: t})
}

func (t *TextMD) Remove(data interface{}) {
	t.Session.removeItem(t.Id)
	t.Driver.Remove(t.Id)
}

// Shell is a Go code cell evaluated with gomacro. Its outputs are persisted
// with the notebook, so results survive reloads.
type Shell struct {
	Driver      *liveview.ComponentDriver[*Shell] `json:"-"`
	Session     *Session                          `json:"-"`
	Interpreter *fast.Interp                      `json:"-"`
	Debug       bool                              `json:"debug"`
	UUID        string                            `json:"uuid"`
	Id          string                            `json:"id"`
	Code        string                            `json:"code"`
	CaptionPlay string                            `json:"caption_play"`
	Result      string                            `json:"result,omitempty"`
	Out         string                            `json:"out,omitempty"`
	Err         string                            `json:"err,omitempty"`
	Runs        int                               `json:"runs,omitempty"`
	ElapsedMS   int64                             `json:"elapsed_ms,omitempty"`
}

// OutputsHTML renders the outputs section. It is used by the template on
// mount and pushed directly into the DOM after a run, so the editor is never
// re-rendered (no flicker, cursor and scroll preserved).
func (t *Shell) OutputsHTML() string {
	var sb strings.Builder
	if t.Runs > 0 {
		sb.WriteString(`<div class="run-meta">run #` + strconv.Itoa(t.Runs) + ` · ` + formatElapsed(t.ElapsedMS) + `</div>`)
	}
	if t.Err != "" {
		sb.WriteString(`<div class="cell-output cell-error"><span class="output-label">Error</span><pre class="output-body">` + t.Err + `</pre></div>`)
	}
	if t.Result != "" {
		sb.WriteString(`<div class="cell-output cell-result"><span class="output-label">Result</span><div class="output-body">` + t.Result + `</div></div>`)
	}
	if t.Out != "" {
		sb.WriteString(`<div class="cell-output cell-stdout"><span class="output-label">Output</span><pre class="output-body">` + t.Out + `</pre></div>`)
	}
	return sb.String()
}

func formatElapsed(ms int64) string {
	if ms < 1000 {
		return strconv.FormatInt(ms, 10) + "ms"
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func (t *Shell) GetDriver() liveview.LiveDriver {
	return t.Driver
}

func (t *Shell) GetTemplate() string {
	te, _ := liveview.FileToString("lives/shell.html")
	return te
}

func (t *Shell) Start() {
	t.UUID = uuid.New().String()
	t.Driver.Commit()
	t.Session.touch(t.Id, Item{Shell: t})
}

func (t *Shell) Remove(data interface{}) {
	t.Session.removeItem(t.Id)
	t.Driver.Remove(t.Id)
}

func (t *Shell) Click(data interface{}) {
	t.Driver.EvalScript("var c=document.getElementById('" + t.UUID + "_div_code'); if(c){c.classList.add('cell--running')}")
	t.Driver.FillValueById(t.UUID+"_status",
		`<span class="status-dot"></span> Running… <span class="status-note">first import of a package can take a while</span>`)

	t.Code = t.Driver.GetElementById(t.UUID + "_code")
	started := time.Now()
	t.Result, t.Out, t.Err = evalCell(t.Interpreter, t.Code)
	t.ElapsedMS = time.Since(started).Milliseconds()
	t.Runs++

	// update only the outputs: the editor keeps its DOM, cursor and scroll
	t.Driver.FillValueById(t.UUID+"_out", t.OutputsHTML())
	t.Driver.FillValueById(t.UUID+"_status", "")
	t.Driver.EvalScript("var c=document.getElementById('" + t.UUID + "_div_code'); if(c){c.classList.remove('cell--running')}")
	t.Session.mountPendingGrids()
	t.Session.refreshVars()
	t.Session.touch(t.Id, Item{Shell: t})
}

// evalCell evaluates Go code in the interpreter, capturing stdout/stderr and
// panics. It is shared by the interactive cells and the headless runner.
func evalCell(interp *fast.Interp, code string) (result, out, errMsg string) {
	old := os.Stdout // keep backup of the real stdout
	oldErr := os.Stderr
	r, w, _ := os.Pipe()

	os.Stderr = w
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		if rec := recover(); rec != nil {
			errMsg = html.EscapeString(fmt.Sprint(rec))
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
		out = html.EscapeString(<-outC)
	}()

	values, types := interp.Eval(code)
	for i, value := range values {
		v := fmt.Sprint(value.ReflectValue())
		if strings.HasPrefix(strings.TrimSpace(v), "<div") {
			result += v
			continue
		}
		result += "<div class=\"res-item\"><pre class=\"res-value\">" +
			html.EscapeString(v) + "</pre><span class=\"res-type\">" +
			html.EscapeString(fmt.Sprint(types[i])) + "</span></div>"
	}
	return result, out, errMsg
}
