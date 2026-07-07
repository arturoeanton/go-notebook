package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pd "github.com/arturoeanton/go-pandas"
	"github.com/arturoeanton/go-pandas/dataframe"
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

func TestReadSQL(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	if _, err := sqlExec("sqlite", dsn, "CREATE TABLE sales (region TEXT, amount REAL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlExec("sqlite", dsn, "INSERT INTO sales VALUES ('North',120),('South',95),('North',132)"); err != nil {
		t.Fatal(err)
	}
	df, err := readSQL("sqlite", dsn, "SELECT region, SUM(amount) AS total FROM sales GROUP BY region ORDER BY total DESC")
	if err != nil {
		t.Fatal(err)
	}
	if df.Len() != 2 {
		t.Fatalf("expected 2 rows, got %d", df.Len())
	}
	cols := df.Columns()
	if len(cols) != 2 || cols[0] != "region" || cols[1] != "total" {
		t.Fatalf("unexpected columns: %v", cols)
	}
	rows := df.ToRows()
	if fmt.Sprint(rows[0][0]) != "North" {
		t.Fatalf("expected North first (ordered by total), got %v", rows[0])
	}
}

func TestDataGridViews(t *testing.T) {
	df, err := readSQLTestFrame()
	if err != nil {
		t.Fatal(err)
	}
	g := newDataGrid("g1", df)
	g.PageSize = 10
	if got := len(g.PageRows()); got != 10 {
		t.Fatalf("expected 10 rows on first page, got %d", got)
	}
	if !g.HasNext() || g.HasPrev() {
		t.Fatal("expected next page only")
	}
	g.SortCol, g.SortAsc = 1, false // score desc
	first := g.PageRows()[0]
	if first[1] != "99" {
		t.Fatalf("expected max score 99 first, got %v", first)
	}
	g.FilterText = "item-01"
	g.Page = 0
	if got := len(g.PageRows()); got != 10 { // item-010..item-019
		t.Fatalf("expected 10 filtered rows, got %d", got)
	}
	if !strings.Contains(g.PageInfo(), "filtered") {
		t.Fatalf("PageInfo should mention filtering: %s", g.PageInfo())
	}
}

func readSQLTestFrame() (*dataframe.DataFrame, error) {
	recs := make([]map[string]any, 0, 100)
	for i := 0; i < 100; i++ {
		recs = append(recs, map[string]any{
			"name":  fmt.Sprintf("item-%03d", i),
			"score": i % 100,
		})
	}
	return pd.DataFrameFromRecords(recs, pd.WithColumnOrder("name", "score"))
}

func TestGridSnippetPlaceholder(t *testing.T) {
	interp := newTestInterpreter(t)
	declareHostFuncs(interp, newSession("test", interp))
	out := evalSnippet(t, interp, "snippet/grid.gonote")
	if !strings.HasPrefix(out, `<div class="grid-slot"`) || !strings.Contains(out, "<table>") {
		t.Fatalf("expected grid placeholder with static snapshot, got: %q", out[:200])
	}
}

func TestExtraCharts(t *testing.T) {
	interp := newTestInterpreter(t)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("chart helpers panicked: %v", r)
		}
	}()
	values, _ := interp.Eval(`
xs := []float64{1, 2, 3, 4, 5}
ys := []float64{2, 4, 3, 5, 6}
ScatterChart(xs, ys, "s") + Histogram(ys, 3, "h") +
	PieChart([]string{"a", "b"}, []float64{3, 7}, "p") +
	AreaChart([]string{"x1", "x2", "x3"}, []string{"s1"}, [][]float64{{1, 3, 2}}, "a")
`)
	last := ""
	for _, v := range values {
		last = fmt.Sprint(v.ReflectValue())
	}
	if got := strings.Count(last, "<svg"); got != 4 {
		t.Fatalf("expected 4 charts, got %d", got)
	}
	if !strings.Contains(last, "<title>") || !strings.Contains(last, "polygon") {
		t.Fatal("expected tooltips and area fill in chart output")
	}
}

func TestSanitizeNotebookName(t *testing.T) {
	cases := map[string]string{
		"default":                 "default",
		"../../etc/passwd":        "passwd",
		"ventas 2026":             "ventas 2026",
		"a/b/c.gonote.json":       "c",
		"<script>":                "script",
		"..":                      "",
		"":                        "",
		"reporte-mensual_v2":      "reporte-mensual_v2",
		"notebooks/x.gonote.json": "x",
		"nombre'; drop table --":  "nombre drop table --",
	}
	for in, want := range cases {
		if got := sanitizeNotebookName(in); got != want {
			t.Errorf("sanitizeNotebookName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChartsSnippet(t *testing.T) {
	interp := newTestInterpreter(t)
	out := evalSnippet(t, interp, "snippet/charts.gonote")
	if !strings.HasPrefix(out, "<div") {
		t.Fatalf("charts cell should return raw HTML, got: %q", out)
	}
	if svgs := strings.Count(out, "<svg"); svgs != 2 {
		t.Fatalf("expected 2 SVG charts, got %d in: %q", svgs, out[:200])
	}
	if !strings.Contains(out, "<table>") || !strings.Contains(out, "polyline") ||
		!strings.Contains(out, "North") {
		t.Fatalf("expected table, bar and line chart with data, got: %q", out[:300])
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
