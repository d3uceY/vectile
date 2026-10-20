package db

import "testing"

func TestFoldString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"already", "ALREADY"},
		{"Mixed CASE 123", "MIXED CASE 123"},
		{"café", "CAFÉ"},
		{"ⱥⱥ", "ȺȺ"},     // U+2C65 folds down to U+023A, not the other way round
		{"Σσς", "ΣΣΣ"},  // a final sigma folds with sigma
		{"\u212A", "K"}, // the Kelvin sign folds with K
		{"Straße", "STRAßE"},
	}
	for _, c := range cases {
		if got := foldString(c.in); got != c.want {
			t.Errorf("foldString(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVFoldRegisteredInSQL(t *testing.T) {
	conn := testDB(t)
	var got string
	if err := conn.QueryRow("SELECT vfold(?)", "Straße ⱥ").Scan(&got); err != nil {
		t.Fatalf("vfold: %v", err)
	}
	if got != "STRAßE Ⱥ" {
		t.Fatalf("vfold = %q", got)
	}
}
