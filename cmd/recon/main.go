package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
	"reconciliation/internal/domain"
	"reconciliation/internal/ingest"
	"reconciliation/internal/mapping"
	"reconciliation/internal/money"
	"reconciliation/internal/reconcile"
	"reconciliation/internal/report"
	"reconciliation/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if err := run(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "migrate":
		if err := migrate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "profile":
		if err := profile(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "patch-config":
		if err := patchConfig(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "import-config":
		if err := importConfig(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "verify":
		if err := verify(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "verify-db":
		if err := verifyDB(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	case "explain":
		if err := explain(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "recon:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func verify(args []string) error {
	f := flag.NewFlagSet("verify", flag.ContinueOnError)
	file := f.String("report", "", "generated after-fix workbook")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *file == "" {
		return fmt.Errorf("--report is required")
	}
	if err := report.Verify(*file); err != nil {
		return err
	}
	fmt.Printf("verified %s\n", *file)
	return nil
}

func verifyDB(args []string) error {
	f := flag.NewFlagSet("verify-db", flag.ContinueOnError)
	url := f.String("postgres-url", os.Getenv("DATABASE_URL"), "PostgreSQL URL")
	runID := f.Int64("run-id", 0, "persisted run id")
	expected := f.Int64("expected-rows", 0, "optional exact source-row count")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *url == "" || *runID <= 0 {
		return fmt.Errorf("--postgres-url and positive --run-id are required")
	}
	p, err := store.Open(context.Background(), *url)
	if err != nil {
		return err
	}
	defer p.Close()
	v, err := store.VerifyRun(context.Background(), p, *runID, *expected)
	if err != nil {
		return err
	}
	fmt.Printf("run=%d mode=%s stage=%s status=%s source_rows=%d payment_rows=%d settlement_rows=%d metadata_rows=%d transaction_rows=%d mappings=%d contributions=%d groups=%d members=%d issues=%d unmatched=%d amount_mismatches=%d bucket_mismatches=%d summary_mismatches=%d header_mismatches=%d\n", v.RunID, v.Mode, v.Stage, v.Status, v.SourceRows, v.PaymentRows, v.SettlementRows, v.MetadataRows, v.TransactionRows, v.MappingRows, v.ContributionRows, v.GroupRows, v.MemberRows, v.IssueRows, v.UnmatchedRows, v.AmountMismatches, v.BucketMismatches, v.SummaryMismatches, v.HeaderMismatches)
	return nil
}

func explain(args []string) error {
	f := flag.NewFlagSet("explain", flag.ContinueOnError)
	url := f.String("postgres-url", os.Getenv("DATABASE_URL"), "PostgreSQL URL")
	runID := f.Int64("run-id", 0, "persisted run id")
	recordRef := f.String("record-ref", "", "exact mapped record reference")
	field := f.String("summary-field", "", "summary target field")
	line := f.Int("source-line", 0, "physical source line")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *url == "" || *runID <= 0 {
		return fmt.Errorf("--postgres-url and positive --run-id are required")
	}
	p, err := store.Open(context.Background(), *url)
	if err != nil {
		return err
	}
	defer p.Close()
	rows, err := store.Explain(context.Background(), p, *runID, *recordRef, *field, *line)
	if err != nil {
		return err
	}
	fmt.Println("source\tphysical_line\tordinal\tscope\tsettlement_id\tcurrency\trecord_ref\tamount_field\tamount\ttarget\tdecision\trule_file\trule_line")
	for _, row := range rows {
		fmt.Printf("%s\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n", row.Source, row.PhysicalLine, row.Ordinal, row.Scope, row.SettlementID, row.Currency, row.RecordRef, row.AmountField, row.Amount, row.Target, row.Decision, row.RuleFile, row.RuleLine)
	}
	return nil
}

func patchConfig(args []string) error {
	f := flag.NewFlagSet("patch-config", flag.ContinueOnError)
	in := f.String("input", "", "original config CSV")
	patch := f.String("patch", "fixes/mapping_fixes.csv", "data-driven patch CSV")
	out := f.String("output", "", "patched config CSV")
	source := f.String("source", "", "payment or settlement")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *in == "" || *out == "" || *source == "" {
		return fmt.Errorf("--input, --output and --source are required")
	}
	if *source != string(domain.SourcePayment) && *source != string(domain.SourceSettlement) {
		return fmt.Errorf("--source must be payment or settlement")
	}
	ops, err := mapping.LoadPatch(*patch)
	if err != nil {
		return err
	}
	if err := mapping.ApplyPatch(*in, *out, *source, ops); err != nil {
		return err
	}
	fmt.Printf("wrote patched config %s\n", *out)
	return nil
}

func importConfig(args []string) error {
	f := flag.NewFlagSet("import-config", flag.ContinueOnError)
	pc := f.String("payment-config", "", "payment mapping CSV")
	sc := f.String("settlement-config", "", "settlement mapping CSV")
	name := f.String("name", "", "frozen config version name")
	note := f.String("note", "imported by recon", "config version note")
	url := f.String("postgres-url", os.Getenv("DATABASE_URL"), "PostgreSQL URL")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *pc == "" || *sc == "" || *url == "" {
		return fmt.Errorf("--payment-config, --settlement-config and --postgres-url or DATABASE_URL are required")
	}
	pr, err := mapping.LoadConfig(*pc, domain.SourcePayment)
	if err != nil {
		return err
	}
	sr, err := mapping.LoadConfig(*sc, domain.SourceSettlement)
	if err != nil {
		return err
	}
	pi, err := store.FileInfoForPath(*pc)
	if err != nil {
		return err
	}
	si, err := store.FileInfoForPath(*sc)
	if err != nil {
		return err
	}
	p, err := store.Open(context.Background(), *url)
	if err != nil {
		return err
	}
	defer p.Close()
	if err := store.Migrate(context.Background(), p); err != nil {
		return err
	}
	version, err := store.EnsureFrozenConfig(context.Background(), p, *name, *note, pi, si, append(pr, sr...))
	if err != nil {
		return err
	}
	fmt.Printf("config_version=%d name=%s content_sha256=%s\n", version.ID, version.Name, version.ContentSHA256)
	return nil
}

func migrate(args []string) error {
	f := flag.NewFlagSet("migrate", flag.ContinueOnError)
	url := f.String("postgres-url", os.Getenv("DATABASE_URL"), "PostgreSQL URL")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *url == "" {
		return fmt.Errorf("--postgres-url or DATABASE_URL is required")
	}
	p, err := store.Open(context.Background(), *url)
	if err != nil {
		return err
	}
	defer p.Close()
	if err := store.Migrate(context.Background(), p); err != nil {
		return err
	}
	fmt.Println("migrations applied")
	return nil
}
func usage() {
	fmt.Println("recon run --payments FILE --settlements FILE --settlement-id ID --output FILE [--payment-config FILE --settlement-config FILE | --config-version NAME] [--mode diagnostic-baseline|strict]")
}
func run(args []string) error {
	f := flag.NewFlagSet("run", flag.ContinueOnError)
	payments := f.String("payments", "", "payments CSV")
	settlements := f.String("settlements", "", "settlements TSV")
	pc := f.String("payment-config", "", "payment mapping CSV")
	sc := f.String("settlement-config", "", "settlement mapping CSV")
	sid := f.String("settlement-id", "", "settlement id")
	output := f.String("output", "reconciliation.xlsx", "output workbook")
	mode := f.String("mode", "diagnostic-baseline", "mapping mode")
	pgurl := f.String("postgres-url", os.Getenv("DATABASE_URL"), "optional PostgreSQL URL")
	runName := f.String("run-name", "local-run", "run name when persisting")
	configVersionName := f.String("config-version", "", "frozen database config version name; requires PostgreSQL")
	retry := f.Bool("retry", false, "retry a previously failed persisted run with the same fingerprint")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *mode != string(mapping.Diagnostic) && *mode != string(mapping.Strict) {
		return fmt.Errorf("--mode must be %q or %q", mapping.Diagnostic, mapping.Strict)
	}
	for k, v := range map[string]string{"payments": *payments, "settlements": *settlements, "settlement-id": *sid} {
		if v == "" {
			return fmt.Errorf("--%s is required", k)
		}
	}
	if *configVersionName == "" {
		for k, v := range map[string]string{"payment-config": *pc, "settlement-config": *sc} {
			if v == "" {
				return fmt.Errorf("--%s is required unless --config-version is supplied", k)
			}
		}
	}
	for _, input := range []string{*payments, *settlements, *pc, *sc} {
		if input != "" {
			same, pathErr := samePath(input, *output)
			if pathErr != nil {
				return pathErr
			}
			if same {
				return fmt.Errorf("--output must not overwrite input %s", input)
			}
		}
	}
	var pool *pgxpool.Pool
	var err error
	if *pgurl != "" {
		pool, err = store.Open(context.Background(), *pgurl)
		if err != nil {
			return err
		}
		defer pool.Close()
		if err = store.Migrate(context.Background(), pool); err != nil {
			return err
		}
	}
	p, pi, err := ingest.ParsePayments(*payments)
	if err != nil {
		return err
	}
	p = reconcile.Classify(p, *sid)
	paymentRowCount := len(p)
	memoryMark("payments-parsed")
	var pr, sr []domain.ConfigRule
	var configVersion store.FrozenConfig
	var paymentConfigFile, settlementConfigFile store.FileInfo
	if *configVersionName != "" {
		if pool == nil {
			return fmt.Errorf("--config-version requires --postgres-url or DATABASE_URL")
		}
		configVersion, err = store.LookupConfigVersion(context.Background(), pool, *configVersionName)
		if err != nil {
			return err
		}
		rules, loadErr := store.LoadRules(context.Background(), pool, configVersion.ID)
		if loadErr != nil {
			return loadErr
		}
		for _, rule := range rules {
			switch rule.Source {
			case domain.SourcePayment:
				pr = append(pr, rule)
			case domain.SourceSettlement:
				sr = append(sr, rule)
			}
		}
		if len(pr) == 0 || len(sr) == 0 {
			return fmt.Errorf("config version %q has incomplete payment/settlement rules", *configVersionName)
		}
	} else {
		pr, err = mapping.LoadConfig(*pc, domain.SourcePayment)
		if err != nil {
			return err
		}
		sr, err = mapping.LoadConfig(*sc, domain.SourceSettlement)
		if err != nil {
			return err
		}
		if pool != nil {
			paymentConfig, infoErr := store.FileInfoForPath(*pc)
			if infoErr != nil {
				return infoErr
			}
			settlementConfig, infoErr := store.FileInfoForPath(*sc)
			if infoErr != nil {
				return infoErr
			}
			paymentConfigFile, settlementConfigFile = paymentConfig, settlementConfig
			allConfigRules := append(append([]domain.ConfigRule{}, pr...), sr...)
			configVersion, err = store.EnsureFrozenConfig(context.Background(), pool, "", "automatically imported by recon run", paymentConfig, settlementConfig, allConfigRules)
			if err != nil {
				return err
			}
		}
		if paymentConfigFile.SHA256 == "" {
			paymentConfigFile, err = store.FileInfoForPath(*pc)
			if err != nil {
				return err
			}
			settlementConfigFile, err = store.FileInfoForPath(*sc)
			if err != nil {
				return err
			}
		}
	}
	m := mapping.Diagnostic
	if *mode == string(mapping.Strict) {
		m = mapping.Strict
	}
	pe, psum, pissues, err := (mapping.Engine{Rules: pr, Mode: m}).Map(p)
	if err != nil {
		return err
	}
	p = nil
	releaseRawMaps(pe)
	runtime.GC()
	memoryMark("payments-mapped-released")
	s, si, err := ingest.ParseSettlements(*settlements)
	if err != nil {
		return err
	}
	s = reconcile.Classify(s, *sid)
	settlementRowCount := len(s)
	memoryMark("settlements-parsed")
	se, ssum, sissues, err := (mapping.Engine{Rules: sr, Mode: m}).Map(s)
	if err != nil {
		return err
	}
	groups, err := reconcile.BuildGroups(pe, se)
	if err != nil {
		return err
	}
	memoryMark("groups-built")
	header := int64(0)
	currency := ""
	metadataCount := 0
	for _, r := range s {
		if r.Kind == domain.RowMetadata && r.SettlementID == *sid {
			metadataCount++
			header = r.ReconAmount
			currency = r.Currency
		}
	}
	if metadataCount != 1 {
		return fmt.Errorf("selected settlement %q has %d metadata rows, want exactly 1", *sid, metadataCount)
	}
	var settlementActivity int64
	for _, row := range s {
		if row.Kind == domain.RowTransaction && row.Scope == domain.ScopeIn {
			settlementActivity, err = money.Add(settlementActivity, row.ReconAmount)
			if err != nil {
				return err
			}
		}
	}
	if header != settlementActivity {
		return fmt.Errorf("settlement activity %s does not equal metadata header %s", money.FormatCents(settlementActivity), money.FormatCents(header))
	}
	s = nil
	releaseRawMaps(se)
	runtime.GC()
	memoryMark("settlements-mapped-released")
	issues := append(pissues, sissues...)
	result := domain.RunResult{PaymentRows: pe, SettlementRows: se, Groups: groups, Summary: mergeSummary(psum, ssum), SettlementHeader: header, SettlementCurrency: currency, Issues: issues}
	meta := map[string]string{"payments_file": filepath.Base(pi.Path), "payments_sha256": pi.SHA256, "settlements_file": filepath.Base(si.Path), "settlements_sha256": si.SHA256, "settlement_id": *sid, "mode": *mode, "payment_rows": fmt.Sprint(paymentRowCount), "settlement_rows": fmt.Sprint(settlementRowCount), "mapping_issues": fmt.Sprint(len(issues))}
	if configVersion.ID > 0 {
		meta["config_version_id"] = fmt.Sprint(configVersion.ID)
		meta["config_version_name"] = configVersion.Name
		meta["config_version_sha256"] = configVersion.ContentSHA256
	}
	if paymentConfigFile.SHA256 != "" {
		meta["payment_config_file"] = filepath.Base(paymentConfigFile.Path)
		meta["payment_config_sha256"] = paymentConfigFile.SHA256
		meta["settlement_config_file"] = filepath.Base(settlementConfigFile.Path)
		meta["settlement_config_sha256"] = settlementConfigFile.SHA256
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil && filepath.Dir(*output) != "." {
		return err
	}
	if err := report.Write(*output, result, meta); err != nil {
		return err
	}
	if m == mapping.Strict {
		if err := report.Verify(*output); err != nil {
			return fmt.Errorf("verify generated report: %w", err)
		}
	}
	memoryMark("report-written")
	if pool != nil {
		allRules := append(append([]domain.ConfigRule{}, pr...), sr...)
		runID, err := store.Persist(context.Background(), pool, *runName, *sid, *mode, configVersion.ID, *retry, store.FileInfo{Path: pi.Path, SHA256: pi.SHA256, Bytes: pi.Bytes}, store.FileInfo{Path: si.Path, SHA256: si.SHA256, Bytes: si.Bytes}, pe, se, groups, result.Summary, allRules, issues)
		if err != nil {
			return err
		}
		if err := store.RegisterReport(context.Background(), pool, runID, "reconciliation", *output); err != nil {
			return err
		}
		fmt.Printf("persisted PostgreSQL run_id=%d\n", runID)
	}
	fmt.Printf("wrote %s: payment_rows=%d settlement_rows=%d groups=%d issues=%d\n", *output, paymentRowCount, settlementRowCount, len(groups), len(issues))
	return nil
}

func memoryMark(label string) {
	if os.Getenv("RECON_MEM_PROFILE") == "" {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(os.Stderr, "memory stage=%s heap_alloc_mb=%.1f heap_inuse_mb=%.1f sys_mb=%.1f\n", label, float64(m.HeapAlloc)/(1024*1024), float64(m.HeapInuse)/(1024*1024), float64(m.Sys)/(1024*1024))
}

func releaseRawMaps(rows []domain.MappedRow) {
	for i := range rows {
		if rows[i].Row.Kind == domain.RowMetadata {
			continue
		}
		rows[i].Row.Raw = nil
		rows[i].Row.Canonical = nil
	}
}
func mergeSummary(a, b domain.Summary) domain.Summary {
	out := domain.Summary{Buckets: map[string]map[domain.Source]int64{}}
	for k, m := range a.Buckets {
		out.Buckets[k] = map[domain.Source]int64{domain.SourcePayment: m[domain.SourcePayment], domain.SourceSettlement: m[domain.SourceSettlement]}
	}
	for k, m := range b.Buckets {
		if out.Buckets[k] == nil {
			out.Buckets[k] = map[domain.Source]int64{}
		}
		out.Buckets[k][domain.SourceSettlement] = m[domain.SourceSettlement]
	}
	return out
}
func samePath(a, b string) (bool, error) {
	aa, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return filepath.Clean(aa) == filepath.Clean(bb), nil
}
func profile(args []string) error {
	f := flag.NewFlagSet("profile", flag.ContinueOnError)
	p := f.String("payments", "", "payments CSV")
	s := f.String("settlements", "", "settlements TSV")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	if *p == "" || *s == "" {
		return fmt.Errorf("--payments and --settlements are required")
	}
	pp, pi, e := ingest.ParsePayments(*p)
	if e != nil {
		return e
	}
	ss, si, e := ingest.ParseSettlements(*s)
	if e != nil {
		return e
	}
	fmt.Printf("payments=%d bytes=%d sha256=%s\nsettlements=%d bytes=%d sha256=%s\n", len(pp), pi.Bytes, pi.SHA256, len(ss), si.Bytes, si.SHA256)
	return nil
}
