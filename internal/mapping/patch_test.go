package mapping

import (
	"os"
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

func TestPatchValidationIsGuardedAndAtomic(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(bad, []byte("source,origin_line\npayment,2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPatch(bad); err == nil {
		t.Fatal("missing patch headers accepted")
	}
	input := filepath.Join(dir, "config.csv")
	output := filepath.Join(dir, "out.csv")
	data := "transaction_type,record_ref,to_summary_field_when_positive_amount,to_summary_field_when_negative_amount\nORDER,id,old,old\n"
	if err := os.WriteFile(input, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ops := []PatchOperation{{Source: "payment", Line: 99, Action: "update", OldPositive: "old", OldNegative: "old", NewPositive: "new", NewNegative: "new"}}
	if err := ApplyPatch(input, output, "payment", ops); err == nil {
		t.Fatal("missing target line accepted")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed patch published output: %v", err)
	}
	if err := ApplyPatch(input, input, "payment", ops); err == nil {
		t.Fatal("in-place overwrite accepted")
	}
}
