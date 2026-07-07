package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gomarkdown/markdown"
)

// runHeadless executes a notebook from the command line and writes a
// standalone HTML report:
//
//	go-notebook run <notebook> [-p key=value]... [-o report.html]
//
// Parameters are declared in the interpreter as string variables before any
// cell runs, so notebooks can be parameterized (papermill style).
func runHeadless(args []string) int {
	var params []string
	var output string
	var name string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			i++
			if i < len(args) {
				params = append(params, args[i])
			}
		case "-o":
			i++
			if i < len(args) {
				output = args[i]
			}
		default:
			if name == "" {
				name = args[i]
			}
		}
	}

	name = sanitizeNotebookName(name)
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: go-notebook run <notebook> [-p key=value]... [-o report.html]")
		return 2
	}
	data, err := os.ReadFile(notebookPath(name))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		fmt.Fprintln(os.Stderr, "error parsing notebook:", err)
		return 1
	}
	if output == "" {
		output = name + ".html"
	}

	interp := newInterpreter()
	session := newSession(name, interp)
	declareHostFuncs(interp, session)

	paramValues := make(map[string]string, len(params))
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			fmt.Fprintf(os.Stderr, "error: invalid parameter %q (want key=value)\n", p)
			return 2
		}
		interp.DeclVar(k, nil, v)
		paramValues[k] = v
	}
	interp.DeclVar("Params", nil, paramValues)

	failures := 0
	ran := 0
	for _, key := range note.Order {
		item := note.Items[key]
		if item.MD != nil && item.MD.Result == "" && item.MD.CodeMD != "" {
			item.MD.Result = string(markdown.ToHTML([]byte(item.MD.CodeMD), nil, nil))
		}
		if item.Shell == nil {
			continue
		}
		ran++
		fmt.Fprintf(os.Stderr, "running %s… ", key)
		result, out, errMsg := evalCell(interp, item.Shell.Code)
		item.Shell.Result, item.Shell.Out, item.Shell.Err = result, out, errMsg
		if errMsg != "" {
			failures++
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
			continue
		}
		fmt.Fprintln(os.Stderr, "ok")
	}

	if err := os.WriteFile(output, []byte(renderExport(name, note)), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing report:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "%d cells run, %d failed — report: %s\n", ran, failures, output)
	if failures > 0 {
		return 1
	}
	return 0
}
