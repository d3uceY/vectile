package parser

import (
	"strings"
	"testing"
)

func TestParseCSVText(t *testing.T) {
	csv := "name,role,team\n\"Smith, John\",CFO,Finance\nAlice,CTO,Engineering\n\n"
	got := ParseCSVText(writeTempFile(t, "table.csv", csv))
	for _, want := range []string{"name | role | team", "Smith, John | CFO | Finance", "Alice | CTO | Engineering"} {
		if !strings.Contains(got, want) {
			t.Fatalf("csv text missing %q:\n%s", want, got)
		}
	}
}

func TestParseJSONText(t *testing.T) {
	// Minified JSON must come back pretty-printed so word chunking works.
	got := ParseJSONText(writeTempFile(t, "data.json", `{"a":1,"b":[1,2,{"c":"d"}]}`))
	if !strings.Contains(got, "\n  \"a\": 1") {
		t.Fatalf("json not pretty-printed:\n%s", got)
	}
	if !strings.Contains(got, "\"c\": \"d\"") {
		t.Fatalf("json content missing after pretty-print:\n%s", got)
	}

	// Invalid JSON falls back to the raw content.
	raw := "{oops not json"
	got = ParseJSONText(writeTempFile(t, "bad.json", raw))
	if strings.TrimSpace(got) != raw {
		t.Fatalf("invalid json should index raw, got %q", got)
	}
}

func TestParseXMLFile(t *testing.T) {
	doc := ParseXMLFile(writeTempFile(t, "note.xml",
		`<?xml version="1.0"?><note xmlns:x="http://x"><to>Tove</to><from>Jani</from></note>`))
	if doc.RootElement != "note" {
		t.Fatalf("root element = %q, want note", doc.RootElement)
	}
	if !strings.Contains(doc.Text, "Tove") {
		t.Fatalf("xml text missing content: %q", doc.Text)
	}
}

func TestParseSQLFile(t *testing.T) {
	doc := ParseSQLFile(writeTempFile(t, "schema.sql",
		"CREATE TABLE IF NOT EXISTS users (id INT);\n"+
			"CREATE TABLE orders (id INT);\n"+
			"INSERT INTO orders VALUES (1);\n"+
			"SELECT * FROM users JOIN orders ON users.id = orders.id;\n"+
			"DROP TABLE legacy;"))
	for _, want := range []string{"users", "orders"} {
		if !containsStr(doc.Tables, want) {
			t.Fatalf("tables missing %q: %v", want, doc.Tables)
		}
	}
	for _, want := range []string{"CREATE", "INSERT", "SELECT", "DROP"} {
		if !containsStr(doc.Statements, want) {
			t.Fatalf("statements missing %q: %v", want, doc.Statements)
		}
	}
	if len(doc.Tables) != 2 {
		t.Fatalf("expected de-duplicated tables, got %v", doc.Tables)
	}
	if !strings.Contains(doc.Text, "CREATE TABLE") {
		t.Fatalf("sql raw text missing: %q", doc.Text)
	}
}

func TestParseShellFile(t *testing.T) {
	doc := ParseShellFile(writeTempFile(t, "run.sh",
		"#!/bin/bash\n\nfoo() {\n  echo hi\n}\n\nfunction bar {\n  echo no\n}\n\nbaz () {\n  echo yes\n}\n"))
	for _, want := range []string{"foo", "baz"} {
		if !containsStr(doc.Functions, want) {
			t.Fatalf("functions missing %q: %v", want, doc.Functions)
		}
	}
	if containsStr(doc.Functions, "bar") {
		t.Fatalf("non-parenthesized function should not match: %v", doc.Functions)
	}
	if !strings.Contains(doc.Text, "function bar") {
		t.Fatalf("shell raw text missing: %q", doc.Text)
	}
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
