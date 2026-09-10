package mapping

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
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
		idx[strings.TrimSpace(h)] = i
	}
	var out []PatchOperation
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		get := func(k string) string {
			if i := idx[k]; i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		line, e := strconv.Atoi(get("origin_line"))
		if e != nil {
			return nil, fmt.Errorf("patch line %q: %w", get("origin_line"), e)
		}
		out = append(out, PatchOperation{Source: get("source"), Line: line, Action: get("action"), OldPositive: get("old_positive"), OldNegative: get("old_negative"), NewPositive: get("new_positive"), NewNegative: get("new_negative")})
	}
	return out, nil
}

func ApplyPatch(input, output string, source string, ops []PatchOperation) error {
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
		idx[strings.TrimSpace(h)] = i
	}
	pby := map[int]PatchOperation{}
	for _, op := range ops {
		if op.Source == source {
			pby[op.Line] = op
		}
	}
	rows := [][]string{head}
	line := 1
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		line++
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
			continue
		}
		if op.Action != "update" {
			return fmt.Errorf("unsupported patch action %q", op.Action)
		}
		row[pos] = op.NewPositive
		row[neg] = op.NewNegative
		rows = append(rows, row)
	}
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()
	w := csv.NewWriter(out)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
