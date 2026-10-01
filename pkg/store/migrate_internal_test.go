package store

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/altenwald/backlog/pkg/model"
)

// A database created before spec pages had links gets a main page and
// derived links when upgraded.
func TestMigrateSpecToWiki(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, dbFileName)

	all := migrations
	migrations = all[:1]
	db, err := openDB(path)
	migrations = all
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`INSERT INTO projects(slug, name, inserted_at, updated_at) VALUES('p', 'P', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
		`INSERT INTO spec_sections VALUES('p', 'overview', 0, 'Overview', 'Intro', '2025-01-01T00:00:00Z')`,
		`INSERT INTO spec_sections VALUES('p', 'scope', 1, 'Scope', 'See [intro](spec:main)', '2025-01-01T00:00:00Z')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	st, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	infos, err := st.ListSpecSections("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 || infos[0].ID != model.SpecMainID || infos[1].ID != "scope" {
		t.Fatalf("unexpected pages after upgrade: %+v", infos)
	}
	if !reflect.DeepEqual(infos[0].Links, []string{"scope"}) || !reflect.DeepEqual(infos[1].Links, []string{"main"}) {
		t.Fatalf("links not derived: %+v", infos)
	}
}
