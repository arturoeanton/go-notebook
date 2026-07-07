package main

import (
	"encoding/json"
	"html"
	"net/http"
	"os"
	"strings"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/labstack/echo/v4"
)

// exportHandler renders a saved notebook as a standalone, shareable HTML page:
// markdown cells, code and persisted outputs, with the notebook styles inlined.
func exportHandler(c echo.Context) error {
	name := sanitizeNotebookName(c.Param("name"))
	if name == "" {
		return c.String(http.StatusBadRequest, "invalid notebook name")
	}
	data, err := os.ReadFile(notebookPath(name))
	if err != nil {
		return c.String(http.StatusNotFound, "notebook not found")
	}
	var n Note
	if err := json.Unmarshal(data, &n); err != nil {
		return c.String(http.StatusInternalServerError, "invalid notebook file")
	}

	css, _ := liveview.FileToString("lives/style.css")
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"/>")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"/>")
	sb.WriteString("<title>" + html.EscapeString(name) + " — Go Notebook</title>")
	sb.WriteString("<style>" + css + exportCSS + "</style></head><body>")
	sb.WriteString("<div class=\"export cells\"><h1 class=\"export-title\">" + html.EscapeString(name) + "</h1>")

	for _, key := range n.Order {
		item := n.Items[key]
		if item.MD != nil && item.MD.CodeMD != "" {
			body := item.MD.Result
			if body == "" {
				body = "<pre>" + html.EscapeString(item.MD.CodeMD) + "</pre>"
			}
			sb.WriteString("<div class=\"cell cell-md\"><div class=\"cell-main\"><div class=\"md-body\">" + body + "</div></div></div>")
		}
		if item.Shell != nil {
			sb.WriteString("<div class=\"cell cell-code\"><div class=\"cell-main\">")
			sb.WriteString("<pre class=\"export-code\">" + html.EscapeString(item.Shell.Code) + "</pre>")
			if item.Shell.Err != "" {
				sb.WriteString("<div class=\"cell-output cell-error\"><span class=\"output-label\">Error</span><pre class=\"output-body\">" + item.Shell.Err + "</pre></div>")
			}
			if item.Shell.Result != "" {
				sb.WriteString("<div class=\"cell-output cell-result\"><span class=\"output-label\">Result</span><div class=\"output-body\">" + item.Shell.Result + "</div></div>")
			}
			if item.Shell.Out != "" {
				sb.WriteString("<div class=\"cell-output cell-stdout\"><span class=\"output-label\">Output</span><pre class=\"output-body\">" + item.Shell.Out + "</pre></div>")
			}
			sb.WriteString("</div></div>")
		}
	}
	sb.WriteString("</div></body></html>")
	return c.HTML(http.StatusOK, sb.String())
}

const exportCSS = `
.export { padding-top: 32px; }
.export .cell { padding: 4px 0; }
.export .cell-main { padding: 10px 14px; }
.export-title { letter-spacing: -0.02em; margin: 0 0 10px 4px; }
.export-code {
	margin: 0;
	font-family: var(--mono);
	font-size: 13px;
	line-height: 1.55;
	white-space: pre-wrap;
	word-break: break-word;
}
`
