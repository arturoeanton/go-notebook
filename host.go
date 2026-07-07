package main

import (
	"database/sql"
	"fmt"

	pd "github.com/arturoeanton/go-pandas"
	"github.com/arturoeanton/go-pandas/dataframe"
	"github.com/cosmos72/gomacro/fast"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// declareHostFuncs injects host-side helpers into a session's interpreter.
// They run compiled in the server binary but are callable from any cell.
func declareHostFuncs(interp *fast.Interp, s *Session) {
	interp.DeclFunc("ReadSQL", readSQL)
	interp.DeclFunc("SQLExec", sqlExec)
	interp.DeclFunc("ShowGrid", func(df *dataframe.DataFrame) string {
		return s.registerGrid(df)
	})
}

// readSQL runs a query and returns the result as a DataFrame.
// Drivers compiled in: "sqlite" (modernc, no cgo) and "postgres" (lib/pq).
func readSQL(driver, dsn, query string) (*dataframe.DataFrame, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	records := make([]map[string]any, 0, 64)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := make(map[string]any, len(cols))
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			rec[c] = v
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("query returned no rows")
	}
	return pd.DataFrameFromRecords(records, pd.WithColumnOrder(cols...))
}

// sqlExec runs a statement (DDL/DML) and returns the affected row count.
func sqlExec(driver, dsn, statement string) (int64, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	res, err := db.Exec(statement)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// registerGrid stores a live DataGrid for the session and returns the
// placeholder HTML the cell result embeds. The Shell mounts the grid right
// after committing the result, so the placeholder becomes interactive.
func (s *Session) registerGrid(df *dataframe.DataFrame) string {
	id := "grid_" + uuid.NewString()
	g := newDataGrid(id, df)
	s.mu.Lock()
	s.pendingGrids = append(s.pendingGrids, g)
	s.mu.Unlock()
	return `<div class="grid-slot" id="` + id + `">` + g.StaticSnapshot() + `</div>`
}

// mountPendingGrids attaches the grids created during a cell run to the page.
func (s *Session) mountPendingGrids() {
	s.mu.Lock()
	grids := s.pendingGrids
	s.pendingGrids = nil
	page := s.page
	s.mu.Unlock()
	if page == nil {
		return
	}
	for _, g := range grids {
		drv := newGridDriver(g)
		page.MountWithStart(g.Id, drv)
	}
}
