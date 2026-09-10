package mapping

import (
	"path/filepath"
	"testing"
)

func TestReferencePatchShape(t *testing.T) {
	ops, err := LoadPatch(filepath.Join("..", "..", "fixes", "mapping_fixes.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 7 {
		t.Fatalf("operations=%d", len(ops))
	}
	deletes, updates := 0, 0
	for _, op := range ops {
		if op.Action == "delete" {
			deletes++
		}
		if op.Action == "update" {
			updates++
		}
	}
	if deletes != 2 || updates != 5 {
		t.Fatalf("deletes=%d updates=%d", deletes, updates)
	}
}
