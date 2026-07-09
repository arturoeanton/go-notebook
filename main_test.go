package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

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

func TestDSLRulesSnippet(t *testing.T) {
	interp := newTestInterpreter(t)
	out := evalSnippet(t, interp, "snippet/dsl-rules.gonote")
	if !strings.Contains(out, "<table>") || !strings.Contains(out, "<svg") {
		t.Fatalf("expected table and chart, got: %q", out[:200])
	}
	// amount 145 and 180 get 15%% off; 120 and 99 stay
	if !strings.Contains(out, "123.25") || !strings.Contains(out, "153") {
		t.Fatalf("expected discounted amounts in output, got: %q", out)
	}
	if !strings.Contains(out, "120") || !strings.Contains(out, "99") {
		t.Fatalf("expected untouched amounts in output, got: %q", out)
	}
}

func TestListVars(t *testing.T) {
	interp := newTestInterpreter(t)
	baseline := baselineBinds(interp)
	interp.Eval(`x := 42
mensaje := "hola"`)
	vars := listVars(interp, baseline)
	found := map[string]string{}
	for _, v := range vars {
		found[v.Name] = v.Value
	}
	if found["x"] != "42" {
		t.Fatalf("expected x=42 in vars, got %v", found)
	}
	if found["mensaje"] != "hola" {
		t.Fatalf("expected mensaje=hola in vars, got %v", found)
	}
	if _, ok := found["ShowDF"]; ok {
		t.Fatal("preamble helpers should be hidden by the baseline")
	}
}

func TestRunHeadless(t *testing.T) {
	nb := `{"order":["text_t","shell_t"],"Items":{` +
		`"text_t":{"md":{"uuid":"m","id":"text_t","code_md":"# Reporte","result":"","caption_play":"Edit"}},` +
		`"shell_t":{"shell":{"debug":true,"uuid":"s","id":"shell_t","code":"import \"fmt\"\nfmt.Println(\"saludo:\", quien)\n2+2","caption_play":"Play"}}}}`
	if err := os.WriteFile(notebookPath("__test__"), []byte(nb), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(notebookPath("__test__"))
	outFile := filepath.Join(t.TempDir(), "report.html")

	code := runHeadless([]string{"__test__", "-p", "quien=mundo", "-o", outFile})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	html, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if !strings.Contains(s, "saludo: mundo") {
		t.Fatalf("report should contain parameterized output, got: %.300s", s)
	}
	if !strings.Contains(s, "Reporte") || !strings.Contains(s, "4") {
		t.Fatal("report should contain rendered markdown and the cell result")
	}
}

// fakeLen mimics vg.Length (a named float64) and fakePlot mimics *plot.Plot's
// Save method, so ShowPlot can be tested without a gonum dependency.
type fakeLen float64

type fakePlot struct{ saved string }

func (p *fakePlot) Save(w, h fakeLen, file string) error {
	p.saved = fmt.Sprintf("%vx%v:%s", float64(w), float64(h), file)
	return nil
}

func TestShowPlotReflection(t *testing.T) {
	p := &fakePlot{}
	out := showPlot(p)
	if p.saved != "288x288:static/tmp.png" {
		t.Fatalf("unexpected Save call: %q", p.saved)
	}
	if !strings.HasPrefix(out, "<div><img src='static/tmp.png") {
		t.Fatalf("unexpected ShowPlot output: %q", out)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("ShowPlot with a plain value should panic with a clear message")
		}
	}()
	showPlot(42)
}

func TestAuthTokens(t *testing.T) {
	store := &AuthStore{Secret: "s3cr3t", Users: map[string]string{}}
	tok := store.makeToken("ana", time.Now().Add(time.Hour))
	user, ok := store.verifyToken(tok)
	if !ok || user != "ana" {
		t.Fatalf("expected valid token for ana, got %q %v", user, ok)
	}
	if _, ok := store.verifyToken(tok + "x"); ok {
		t.Fatal("tampered token must be rejected")
	}
	expired := store.makeToken("ana", time.Now().Add(-time.Minute))
	if _, ok := store.verifyToken(expired); ok {
		t.Fatal("expired token must be rejected")
	}
	other := &AuthStore{Secret: "otro"}
	if _, ok := other.verifyToken(tok); ok {
		t.Fatal("token signed with another secret must be rejected")
	}
}

func TestAuthPasswords(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("hunter22"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := &AuthStore{Secret: "s", Users: map[string]string{"ana": string(hash)}}
	if !store.check("ana", "hunter22") {
		t.Fatal("valid password rejected")
	}
	if store.check("ana", "wrong") || store.check("nadie", "hunter22") {
		t.Fatal("invalid credentials accepted")
	}
}

func TestAuthMiddleware(t *testing.T) {
	store := &AuthStore{Secret: "s3cr3t", Users: map[string]string{}}
	e := echo.New()
	e.Use(authMiddleware(store))
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "user="+toString(c.Get("user")))
	})
	e.GET("/login", func(c echo.Context) error { return c.String(http.StatusOK, "login") })

	// unauthenticated GET / redirects to /login
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Fatalf("expected redirect to /login, got %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// authenticated request passes and carries the user
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: authCookie, Value: store.makeToken("ana", time.Now().Add(time.Hour))})
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "user=ana" {
		t.Fatalf("expected user=ana, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestDataSafetyNets(t *testing.T) {
	s := newSession("x", nil)
	s.dir = t.TempDir()

	// opening a missing notebook must not create a file
	if s.load("fantasma") {
		t.Fatal("load of a missing notebook must fail")
	}
	if _, err := os.Stat(s.path("fantasma")); err == nil {
		t.Fatal("load must never create files")
	}

	// delete moves to .trash, recoverable
	os.WriteFile(s.path("importante"), []byte(`{"order":["a"],"Items":{"a":{}}}`), 0644)
	s.trash("importante")
	if _, err := os.Stat(s.path("importante")); err == nil {
		t.Fatal("trash should remove the original file")
	}
	trashed, _ := filepath.Glob(filepath.Join(s.dir, ".trash", "importante-*.gonote.json"))
	if len(trashed) != 1 {
		t.Fatalf("expected the notebook in .trash, got %v", trashed)
	}

	// overwriting a non-empty notebook with an empty note stashes a backup
	os.WriteFile(s.path("x"), []byte(`{"order":["a"],"Items":{"a":{}}}`), 0644)
	s.note = emptyNote()
	s.name = "x"
	s.Flush()
	wiped, _ := filepath.Glob(filepath.Join(s.dir, ".trash", "x-wiped-*.gonote.json"))
	if len(wiped) != 1 {
		t.Fatalf("expected a wipe backup in .trash, got %v", wiped)
	}
	data, _ := os.ReadFile(wiped[0])
	if !strings.Contains(string(data), `"a"`) {
		t.Fatal("wipe backup should hold the previous content")
	}
}

func TestEnsureWorkspace(t *testing.T) {
	oldWD, _ := os.Getwd()
	// run inside a temp copy so workspaces/ does not pollute the repo
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "notebooks"), 0755)
	os.WriteFile(filepath.Join(tmp, "notebooks", "default.gonote.json"), []byte(`{"order":[],"Items":{}}`), 0644)
	os.Chdir(tmp)
	defer os.Chdir(oldWD)

	if dir := ensureWorkspace(""); dir != "notebooks" {
		t.Fatalf("open mode should use notebooks/, got %s", dir)
	}
	dir := ensureWorkspace("ana")
	if dir != filepath.Join("workspaces", "ana") {
		t.Fatalf("unexpected workspace dir: %s", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "default.gonote.json")); err != nil {
		t.Fatal("workspace should be seeded with the default notebook")
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
