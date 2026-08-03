// Package value defines the typed values and rows the engine operates on.
package value

import (
	"math"
	"strconv"
)

type Type int

const (
	TInt Type = iota
	TFloat
	TText
	TBool
)

// Value is a typed, possibly-null scalar. Type is meaningful even when Null,
// so a null column keeps its declared type.
type Value struct {
	Type Type
	Null bool
	I    int64
	F    float64
	S    string
	B    bool
}

// Row is one tuple.
type Row []Value

func NullOf(t Type) Value     { return Value{Type: t, Null: true} }
func Int64(i int64) Value     { return Value{Type: TInt, I: i} }
func Float64(f float64) Value { return Value{Type: TFloat, F: f} }
func Text(s string) Value     { return Value{Type: TText, S: s} }
func Bool(b bool) Value       { return Value{Type: TBool, B: b} }

func (v Value) IsNull() bool { return v.Null }
func (v Value) IsNaN() bool  { return !v.Null && v.Type == TFloat && math.IsNaN(v.F) }

func (v Value) String() string {
	if v.Null {
		return "NULL"
	}
	switch v.Type {
	case TInt:
		return strconv.FormatInt(v.I, 10)
	case TFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	case TText:
		return v.S
	case TBool:
		return strconv.FormatBool(v.B)
	}
	return "NULL"
}
