package services

import (
	"path/filepath"
	"testing"

	"vectile/backend/config"
	"vectile/backend/db"
)

// TestDeleteCollectionKeepsConfiguredSources is the regression test for
// "clear this library" meaning clear the index, not forget the sources: the
// collection row and its indexed data go, the config entry stays, so the
// collection still shows up on Index and a re-index rebuilds it.
func TestDeleteCollectionKeepsConfiguredSources(t *testing.T) {
	if err := db.Open(filepath.Join(t.TempDir(), "delete.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{Projects: map[string][]string{"notes": {filepath.Join("C:", "notes")}}}
	s := &IndexService{core: &Core{Cfg: cfg}}

	if _, err := db.GetOrCreateCollection(db.DB, "notes", "project", nil, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteCollection("notes"); err != nil {
		t.Fatal(err)
	}

	var rows int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM collections WHERE name = 'notes'").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatal("collection row survived the delete")
	}
	if got := s.core.Cfg.Projects["notes"]; len(got) != 1 {
		t.Fatalf("configured sources were dropped: %v", s.core.Cfg.Projects)
	}

	// Deleting again (config-only now, no DB row) is a no-op, not an error.
	if n, err := s.DeleteCollection("notes"); err != nil || n != 0 {
		t.Fatalf("second delete = (%d, %v), want (0, nil)", n, err)
	}
}
