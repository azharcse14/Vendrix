package migrations_test

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"vendrix/backend/internal/db"
	"vendrix/backend/migrations"
)

// TestEmbeddedMigrations (M0 acceptance, 6.1): embed e shudhu migration (tests/ er file na), naam thik, ar
// DEPLOYED e thaka file er sha256 embed kora file er sathe mile (scripts/check-rules.sh er moto, Go diye o).
func TestEmbeddedMigrations(t *testing.T) {
	files, err := db.LoadMigrations(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Version != "001_init" {
		t.Errorf("prothom migration %s", files[0].Version)
	}
	sums := map[string]string{}
	for _, f := range files {
		sums[f.Version+".sql"] = f.SHA256
	}
	fh, err := os.Open("DEPLOYED")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Errorf("DEPLOYED er line: %q", line)
			continue
		}
		if name := strings.TrimPrefix(parts[1], "*"); sums[name] != parts[0] {
			t.Errorf("%s deploy er por bodleche ba nai (6.1)", name)
		}
	}
}
