package mapping

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PatchOperation struct {
	Source      string
	Line        int
	Action      string
	OldPositive string
	OldNegative string
	NewPositive string
	NewNegative string
}

func LoadPatch(path string) ([]PatchOperation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, h := range head {
		h = strings.TrimSpace(h)
		if h == "" {
			return nil, fmt.Errorf("patch has blank header at column %d", i+1)
		}
		if _, ok := idx[h]; ok {
			return nil, fmt.Errorf("patch has duplicate header %q", h)
		}
		idx[h] = i
	}
	for _, h := range []string{"source", "origin_line", "action", "old_positive", "old_negative", "new_positive", "new_negative"} {
		if _, ok := idx[h]; !ok {
			return nil, fmt.Errorf("patch missing %q header", h)
		}
	}
	var out []PatchOperation
	seen := map[string]struct{}{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		physicalLine, _ := r.FieldPos(0)
		if len(row) != len(head) {
			return nil, fmt.Errorf("patch line %d width %d want %d", physicalLine, len(row), len(head))
		}
		get := func(k string) string { return strings.TrimSpace(row[idx[k]]) }
		line, e := strconv.Atoi(get("origin_line"))
		if e != nil {
			return nil, fmt.Errorf("patch line %q: %w", get("origin_line"), e)
		}
		op := PatchOperation{Source: get("source"), Line: line, Action: get("action"), OldPositive: get("old_positive"), OldNegative: get("old_negative"), NewPositive: get("new_positive"), NewNegative: get("new_negative")}
		if op.Source != "payment" && op.Source != "settlement" {
			return nil, fmt.Errorf("patch line %d has invalid source %q", physicalLine, op.Source)
		}
		if op.Line < 2 {
			return nil, fmt.Errorf("patch line %d has invalid origin_line %d", physicalLine, op.Line)
		}
		if op.Action != "update" && op.Action != "delete" {
			return nil, fmt.Errorf("patch line %d has invalid action %q", physicalLine, op.Action)
		}
		key := fmt.Sprintf("%s:%d", op.Source, op.Line)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate patch operation for %s", key)
		}
		seen[key] = struct{}{}
		out = append(out, op)
	}
	return out, nil
}

func ApplyPatch(input, output string, source string, ops []PatchOperation) error {
	if source != "payment" && source != "settlement" {
		return fmt.Errorf("invalid patch source %q", source)
	}
	inAbs, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	outAbs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if inAbs == outAbs {
		return fmt.Errorf("input and output must be different files")
	}
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()
	r := csv.NewReader(in)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return err
	}
	idx := map[string]int{}
	for i, h := range head {
		h = strings.TrimSpace(h)
		if _, ok := idx[h]; ok {
			return fmt.Errorf("config has duplicate header %q", h)
		}
		idx[h] = i
	}
	for _, h := range []string{"to_summary_field_when_positive_amount", "to_summary_field_when_negative_amount"} {
		if _, ok := idx[h]; !ok {
			return fmt.Errorf("config missing %q header", h)
		}
	}
	pby := map[int]PatchOperation{}
	for _, op := range ops {
		if op.Source == source {
			if _, exists := pby[op.Line]; exists {
				return fmt.Errorf("duplicate patch operation for %s line %d", source, op.Line)
			}
			pby[op.Line] = op
		}
	}
	if len(pby) == 0 {
		return fmt.Errorf("patch has no operations for source %q", source)
	}
	applied := map[int]bool{}
	rows := [][]string{head}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		line, _ := r.FieldPos(0)
		if len(row) != len(head) {
			return fmt.Errorf("config line %d width %d want %d", line, len(row), len(head))
		}
		op, ok := pby[line]
		if !ok {
			rows = append(rows, row)
			continue
		}
		pos := idx["to_summary_field_when_positive_amount"]
		neg := idx["to_summary_field_when_negative_amount"]
		if row[pos] != op.OldPositive || row[neg] != op.OldNegative {
			return fmt.Errorf("patch guard failed at config line %d: got (%q,%q)", line, row[pos], row[neg])
		}
		if op.Action == "delete" {
			applied[line] = true
			continue
		}
		if op.Action != "update" {
			return fmt.Errorf("unsupported patch action %q", op.Action)
		}
		row[pos] = op.NewPositive
		row[neg] = op.NewNegative
		applied[line] = true
		rows = append(rows, row)
	}
	for line := range pby {
		if !applied[line] {
			return fmt.Errorf("patch operation targets missing config line %d", line)
		}
	}
	dir := filepath.Dir(outAbs)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	out, err := os.CreateTemp(dir, ".mapping-patch-*.csv")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	w := csv.NewWriter(out)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, outAbs); err != nil {
		return err
	}
	return nil
}
