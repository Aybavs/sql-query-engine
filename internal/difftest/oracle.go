// Package difftest compares this engine against SQLite on generated queries.
// It is test-only: nothing outside its own tests imports it, so the SQLite
// driver never enters the engine's build.
package difftest

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/csv"
	"github.com/aybavs/sql-query-engine/internal/value"

	_ "modernc.org/sqlite"
)

// Row is one result row of driver values (int64, float64, string, or nil).
type Row []any

// Oracle is an in-memory SQLite database loaded with the same fixture data as
// the engine under test, used as the source of truth for query results.
type Oracle struct{ db *sql.DB }

func NewOracle(cat *catalog.Catalog, dataDir string) (*Oracle, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	o := &Oracle{db: db}
	for _, t := range cat.Tables() {
		if err := o.load(t, dataDir); err != nil {
			db.Close()
			return nil, err
		}
	}
	return o, nil
}

func (o *Oracle) load(t *catalog.Table, dataDir string) error {
	cols := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = c.Name + " " + sqliteType(c.Type)
	}
	create := fmt.Sprintf("CREATE TABLE %s (%s)", t.Name, strings.Join(cols, ", "))
	if _, err := o.db.Exec(create); err != nil {
		return fmt.Errorf("create %s: %w", t.Name, err)
	}

	rows, err := csv.Read(filepath.Join(dataDir, t.File), t.Columns)
	if err != nil {
		return err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(t.Columns)), ",")
	insert := fmt.Sprintf("INSERT INTO %s VALUES (%s)", t.Name, placeholders)
	for _, r := range rows {
		args := make([]any, len(r))
		for i, v := range r {
			args[i] = driverValue(v)
		}
		if _, err := o.db.Exec(insert, args...); err != nil {
			return fmt.Errorf("insert into %s: %w", t.Name, err)
		}
	}
	return nil
}

// driverValue converts an engine value into what the SQLite driver expects,
// preserving NULL rather than substituting a zero value.
func driverValue(v value.Value) any {
	if v.IsNull() {
		return nil
	}
	switch v.Type {
	case value.TInt:
		return v.I
	case value.TFloat:
		return v.F
	case value.TBool:
		return v.B
	default:
		return v.S
	}
}

func sqliteType(t value.Type) string {
	switch t {
	case value.TInt:
		return "INTEGER"
	case value.TFloat:
		return "REAL"
	case value.TBool:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

// Query runs a statement and returns its rows as driver values.
func (o *Oracle) Query(query string) ([]Row, error) {
	rs, err := o.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}

	var out []Row
	for rs.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		out = append(out, Row(cells))
	}
	return out, rs.Err()
}

func (o *Oracle) Close() error { return o.db.Close() }
