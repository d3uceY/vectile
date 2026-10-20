package db

import (
	"database/sql/driver"
	"strings"
	"unicode"
	"unicode/utf8"

	"modernc.org/sqlite"
)

// Case folding for substring filters.
//
// SQLite's lower() folds ASCII only, so instr(lower(content), lower(?)) drops
// rows whose match differs in case outside ASCII (ⱥ vs Ⱥ). vfold() folds the way
// Go's (?i) does, so a prefilter never hides a row the match would find.

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("vfold", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) == 1 {
				if s, ok := args[0].(string); ok {
					return foldString(s), nil
				}
			}
			return nil, nil
		})
}

// foldString maps each rune to the smallest in its simple case folding orbit:
// the equivalence (?i) matches on, one rune in and one rune out.
func foldString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	if isASCII(s) {
		// An ASCII letter's orbit is {upper, lower}, so folding is upper-casing:
		// no table lookup per byte on the common path.
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			b.WriteByte(c)
		}
		return b.String()
	}
	for _, r := range s {
		b.WriteRune(foldRune(r))
	}
	return b.String()
}

// foldRune returns the smallest rune in r's case folding orbit.
func foldRune(r rune) rune {
	min := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < min {
			min = f
		}
	}
	return min
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
