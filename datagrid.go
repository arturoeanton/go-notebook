package main

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/arturoeanton/go-pandas/dataframe"
	"github.com/google/uuid"
)

// DataGrid is a live, server-side data explorer for a DataFrame: it keeps the
// rows in memory and pages, sorts and filters them without sending the whole
// dataset to the browser.
type DataGrid struct {
	Driver *liveview.ComponentDriver[*DataGrid] `json:"-"`
	Id     string
	UUID   string

	Cols       []string
	rows       [][]interface{}
	FilterText string
	SortCol    int
	SortAsc    bool
	Page       int
	PageSize   int
}

func newDataGrid(id string, df *dataframe.DataFrame) *DataGrid {
	return &DataGrid{
		Id:       id,
		UUID:     uuid.NewString(),
		Cols:     df.Columns(),
		rows:     df.ToRows(),
		SortCol:  -1,
		PageSize: 10,
	}
}

func (g *DataGrid) GetDriver() liveview.LiveDriver {
	return g.Driver
}

func (g *DataGrid) GetTemplate() string {
	te, _ := liveview.FileToString("lives/datagrid.html")
	return te
}

func (g *DataGrid) Start() {
	g.Driver.Commit()
}

// --- events ------------------------------------------------------------

func (g *DataGrid) Sort(data interface{}) {
	col, err := strconv.Atoi(toString(data))
	if err != nil || col < 0 || col >= len(g.Cols) {
		return
	}
	if g.SortCol == col {
		g.SortAsc = !g.SortAsc
	} else {
		g.SortCol = col
		g.SortAsc = true
	}
	g.Page = 0
	g.Driver.Commit()
}

func (g *DataGrid) Filter(data interface{}) {
	g.FilterText = toString(data)
	g.Page = 0
	g.Driver.Commit()
}

func (g *DataGrid) Prev(data interface{}) {
	if g.Page > 0 {
		g.Page--
	}
	g.Driver.Commit()
}

func (g *DataGrid) Next(data interface{}) {
	if (g.Page+1)*g.PageSize < len(g.view()) {
		g.Page++
	}
	g.Driver.Commit()
}

func (g *DataGrid) Size(data interface{}) {
	if n, err := strconv.Atoi(toString(data)); err == nil && n > 0 && n <= 500 {
		g.PageSize = n
		g.Page = 0
	}
	g.Driver.Commit()
}

// --- template helpers ---------------------------------------------------

func cellString(v interface{}) string {
	return fmt.Sprint(v)
}

func cellFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

// view applies filter and sort over the in-memory rows.
func (g *DataGrid) view() [][]interface{} {
	rows := g.rows
	if f := strings.ToLower(strings.TrimSpace(g.FilterText)); f != "" {
		filtered := make([][]interface{}, 0, len(rows))
		for _, row := range rows {
			for _, v := range row {
				if strings.Contains(strings.ToLower(cellString(v)), f) {
					filtered = append(filtered, row)
					break
				}
			}
		}
		rows = filtered
	}
	if g.SortCol >= 0 && g.SortCol < len(g.Cols) {
		sorted := make([][]interface{}, len(rows))
		copy(sorted, rows)
		col := g.SortCol
		sort.SliceStable(sorted, func(i, j int) bool {
			a, b := sorted[i][col], sorted[j][col]
			fa, oka := cellFloat(a)
			fb, okb := cellFloat(b)
			var less bool
			if oka && okb {
				less = fa < fb
			} else {
				less = cellString(a) < cellString(b)
			}
			if !g.SortAsc {
				return !less
			}
			return less
		})
		rows = sorted
	}
	return rows
}

// Headers returns the column headers with sort indicators, pre-escaped.
func (g *DataGrid) Headers() []map[string]string {
	out := make([]map[string]string, len(g.Cols))
	for i, c := range g.Cols {
		arrow := ""
		if g.SortCol == i {
			if g.SortAsc {
				arrow = " ▲"
			} else {
				arrow = " ▼"
			}
		}
		out[i] = map[string]string{
			"Index": strconv.Itoa(i),
			"Label": html.EscapeString(c) + arrow,
		}
	}
	return out
}

// PageRows returns the current page as pre-escaped strings.
func (g *DataGrid) PageRows() [][]string {
	rows := g.view()
	start := g.Page * g.PageSize
	if start > len(rows) {
		start = len(rows)
	}
	end := start + g.PageSize
	if end > len(rows) {
		end = len(rows)
	}
	out := make([][]string, 0, end-start)
	for _, row := range rows[start:end] {
		r := make([]string, len(row))
		for i, v := range row {
			r[i] = html.EscapeString(cellString(v))
		}
		out = append(out, r)
	}
	return out
}

func (g *DataGrid) PageInfo() string {
	total := len(g.view())
	if total == 0 {
		return "0 rows"
	}
	start := g.Page*g.PageSize + 1
	end := (g.Page + 1) * g.PageSize
	if end > total {
		end = total
	}
	info := fmt.Sprintf("%d–%d of %d", start, end, total)
	if len(g.view()) != len(g.rows) {
		info += fmt.Sprintf(" (filtered from %d)", len(g.rows))
	}
	return info
}

func (g *DataGrid) HasPrev() bool {
	return g.Page > 0
}

func (g *DataGrid) HasNext() bool {
	return (g.Page+1)*g.PageSize < len(g.view())
}

// StaticSnapshot renders the first rows as a plain table. It is what remains
// visible when a notebook is reloaded and the live grid is gone.
func (g *DataGrid) StaticSnapshot() string {
	var sb strings.Builder
	sb.WriteString(`<div class="df-table"><table><thead><tr>`)
	for _, c := range g.Cols {
		sb.WriteString("<th>" + html.EscapeString(c) + "</th>")
	}
	sb.WriteString("</tr></thead><tbody>")
	for i, row := range g.rows {
		if i >= g.PageSize {
			break
		}
		sb.WriteString("<tr>")
		for _, v := range row {
			sb.WriteString("<td>" + html.EscapeString(cellString(v)) + "</td>")
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table>")
	if len(g.rows) > g.PageSize {
		sb.WriteString(fmt.Sprintf(`<div class="dgrid-note">%d rows — run the cell for the interactive grid</div>`, len(g.rows)))
	}
	sb.WriteString("</div>")
	return sb.String()
}
