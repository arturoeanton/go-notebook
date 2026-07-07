package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cosmos72/gomacro/base"
	"github.com/cosmos72/gomacro/fast"
)

// newTestInterpreter builds an interpreter like the server does, but without
// OptModuleImport: the bundled libraries must resolve from the precompiled
// bindings alone, with no network access or plugin compilation.
func newTestInterpreter(t *testing.T) *fast.Interp {
	t.Helper()
	interp := fast.New()
	g := &interp.Comp.Globals
	g.Options |= base.OptKeepUntyped | base.OptTrapPanic
	g.Options &^= base.OptShowPrompt | base.OptShowEval | base.OptShowEvalType

	preamble, err := os.ReadFile("default.gonote")
	if err != nil {
		t.Fatalf("reading default.gonote: %v", err)
	}
	interp.Eval(string(preamble))
	return interp
}

func evalSnippet(t *testing.T, interp *fast.Interp, file string) (last string) {
	t.Helper()
	code, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", file, r)
		}
	}()
	values, _ := interp.Eval(string(code))
	for _, v := range values {
		last = fmt.Sprint(v.ReflectValue())
	}
	return last
}

func TestPandasSnippet(t *testing.T) {
	interp := newTestInterpreter(t)
	out := evalSnippet(t, interp, "snippet/pandas.gonote")
	if !strings.Contains(out, "<table>") || !strings.Contains(out, "Luis") {
		t.Fatalf("expected HTML table with filtered rows, got: %q", out)
	}
	if strings.Contains(out, "Marta") {
		t.Fatalf("Where filter should have dropped Marta, got: %q", out)
	}
}

func TestDSLSnippet(t *testing.T) {
	interp := newTestInterpreter(t)
	out := evalSnippet(t, interp, "snippet/dsl.gonote")
	if out != "42" {
		t.Fatalf("expected DSL result 42, got: %q", out)
	}
}

func TestXMLSnippet(t *testing.T) {
	interp := newTestInterpreter(t)
	code, _ := os.ReadFile("snippet/xml.gonote")
	extra := string(code) + "\ntitle\n"
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("xml snippet panicked: %v", r)
		}
	}()
	values, _ := interp.Eval(extra)
	last := ""
	for _, v := range values {
		last = fmt.Sprint(v.ReflectValue())
	}
	if last != "El Principito" {
		t.Fatalf("expected query result %q, got %q", "El Principito", last)
	}
}

func TestShowDFHelper(t *testing.T) {
	interp := newTestInterpreter(t)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ShowDF panicked: %v", r)
		}
	}()
	values, _ := interp.Eval(`
df, _ := pd.DataFrameFromRecords([]map[string]any{{"a": 1}})
ShowDF(df)
`)
	last := ""
	for _, v := range values {
		last = fmt.Sprint(v.ReflectValue())
	}
	if !strings.HasPrefix(last, "<div class=\"df-table\">") {
		t.Fatalf("ShowDF should return an HTML table, got: %q", last)
	}
}
