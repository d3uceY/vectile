package indexer

import "testing"

// itemProgressName is what the Index view shows under the progress bar for the
// file being indexed. Obsidian and project items carry no Title, so before the
// fallback the real app rendered a blank line where the demo stub showed a
// file name.
func TestItemProgressName(t *testing.T) {
	cases := []struct {
		name string
		item *indexItem
		want string
	}{
		{"title wins when set", &indexItem{Title: "Kubernetes in Action", SourcePath: "libs/k8s.epub"}, "Kubernetes in Action"},
		{"falls back to the file name", &indexItem{SourcePath: "vault/notes/rollout.md"}, "rollout.md"},
		{"repo-relative title", &indexItem{Title: "vectile/backend/db.go", SourcePath: "code/vectile/backend/db.go"}, "vectile/backend/db.go"},
		{"nothing to show", &indexItem{}, ""},
	}
	for _, tc := range cases {
		if got := itemProgressName(tc.item); got != tc.want {
			t.Errorf("%s: itemProgressName() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
