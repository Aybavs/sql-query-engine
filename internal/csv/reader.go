// Package csv reads CSV files into typed rows according to a column schema.
package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func Read(path string, cols []catalog.Column) ([]value.Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = len(cols)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	rows := make([]value.Row, 0, len(records))
	for i, rec := range records {
		row := make(value.Row, len(cols))
		for j, field := range rec {
			v, err := parseField(field, cols[j].Type)
			if err != nil {
				return nil, fmt.Errorf("row %d col %q: %w", i+1, cols[j].Name, err)
			}
			row[j] = v
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseField(s string, t value.Type) (value.Value, error) {
	if s == "" {
		return value.NullOf(t), nil
	}
	switch t {
	case value.TInt:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return value.Value{}, err
		}
		return value.Int64(n), nil
	case value.TFloat:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return value.Value{}, err
		}
		return value.Float64(f), nil
	case value.TBool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return value.Value{}, err
		}
		return value.Bool(b), nil
	default:
		return value.Text(s), nil
	}
}
