package timesten

import (
	"strings"
	"testing"
)

func TestEmbeddedQueriesPassSafetyPolicy(t *testing.T) {
	entries, err := queryFiles.ReadDir("sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := "sql/" + entry.Name()
		query, err := queryFiles.ReadFile(name)
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if err := ValidateDefaultQuery(string(query)); err != nil {
			t.Errorf("%s failed safety policy: %v", name, err)
		}
	}
}
