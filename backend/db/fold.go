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
// SQLite's lower() only case-folds ASCII, so a prefilter written as
// instr(lower(content), lower(?)) silently drops rows whose only match differs
// in case outside ASCII (ⱥ in the text, Ⱥ in the pattern). vfold(text) is
// registered here as a deterministic SQL function that folds text the same way
// Go's (?i) regexp matching does, so a filter and the match it guards agree.

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

// foldString maps every rune to the smallest rune in its Unicode simple case
// folding orbit. Two runes fold together exactly when Go's (?i) regexp matching
// treats them as equal, because both use unicode.SimpleFold. Unlike
// strings.ToLower it maps one rune to one rune, so it never changes a string's
// rune count.
func foldString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	if isASCII(s) {
		// The orbit of an ASCII letter is {upper, lower}, so folding is just
		// upper-casing. This avoids a table lookup per byte on the common path.
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

// foldRune returns the smallest rune in r's simple case folding orbit. Runes
// with no case variant fold to themselves.
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
