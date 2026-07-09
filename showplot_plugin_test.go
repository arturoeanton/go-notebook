package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cosmos72/gomacro/base"
)

func TestDeclFuncAfterPluginImport(t *testing.T) {
	if testing.Short() {
		t.Skip("needs go toolchain + module cache for plugin import")
	}
	interp := newTestInterpreter(t)
	g := &interp.Comp.Globals
	g.Options |= base.OptModuleImport
	declareHostFuncs(interp, newSession("t", interp))

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	vals, _ := interp.Eval(`
import "gonum.org/v1/plot"
p := plot.New()
p.Title.Text = "x"
ShowPlot(p)
`)
	last := ""
	for _, v := range vals {
		last = fmt.Sprint(v.ReflectValue())
	}
	if !strings.Contains(last, "<img") {
		t.Fatalf("expected img from ShowPlot after plugin import, got: %q", last)
	}
}
