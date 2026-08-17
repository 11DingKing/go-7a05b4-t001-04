package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliConfigJSON = `{
  "store_id": "SH001",
  "store_name": "上海旗舰店",
  "trade_date": "2026-08-17",
  "gold_price": 768.50,
  "karat_discount_rules": [
    {"karat": 999, "rate": 0.98},
    {"karat": 990, "rate": 0.95},
    {"karat": 900, "rate": 0.90},
    {"karat": 750, "rate": 0.80}
  ],
  "craft_rate_per_gram": 12.00,
  "currency": "CNY"
}
`

const cliRecordsCSV = `order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
A001,张三,999,2.000,G001,8.000,768.50
A002,李四,990,3.500,G002,7.000,768.50
`

// cliFixture writes a valid store config and a valid two-order export file and
// returns their paths plus a fresh output directory path.
func cliFixture(t *testing.T) (cfgPath, recPath, outDir string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "config.json")
	recPath = filepath.Join(dir, "records.csv")
	outDir = filepath.Join(dir, "out")
	if err := os.WriteFile(cfgPath, []byte(cliConfigJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recPath, []byte(cliRecordsCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, recPath, outDir
}

// TestSettleAllValidOrdersExitsZeroAndWritesOutputs runs the settle subcommand
// on a fully valid daily export and asserts the documented success behaviour:
// exit code 0 and both output files present.
func TestSettleAllValidOrdersExitsZeroAndWritesOutputs(t *testing.T) {
	cfgPath, recPath, outDir := cliFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"settle", "-config", cfgPath, "-input", recPath, "-outdir", outDir},
		strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("settle exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("settle wrote to stderr: %s", stderr.String())
	}
	detail, err := os.ReadFile(filepath.Join(outDir, "detail.csv"))
	if err != nil {
		t.Fatalf("read detail.csv: %v", err)
	}
	if lines := strings.Count(strings.TrimRight(string(detail), "\n"), "\n") + 1; lines != 3 {
		t.Errorf("detail.csv has %d line(s), want 3 (header + 2 orders):\n%s", lines, detail)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var summary struct {
		TotalOrders int `json:"total_orders"`
		ValidOrders int `json:"valid_orders"`
		ErrorOrders int `json:"error_orders"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("parse summary.json: %v", err)
	}
	if summary.TotalOrders != 2 || summary.ValidOrders != 2 || summary.ErrorOrders != 0 {
		t.Errorf("summary = %+v, want total 2 / valid 2 / error 0", summary)
	}
}

// TestSettleWithSingleWorkerExitsZero repeats the same run with -workers 1 so
// the outcome cannot depend on the amount of parallelism.
func TestSettleWithSingleWorkerExitsZero(t *testing.T) {
	cfgPath, recPath, outDir := cliFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"settle", "-config", cfgPath, "-input", recPath, "-outdir", outDir, "-workers", "1"},
		strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("settle -workers 1 exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
}

// TestSettleFromStdinExitsZero covers the "-input -" form documented in README.
func TestSettleFromStdinExitsZero(t *testing.T) {
	cfgPath, _, outDir := cliFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"settle", "-config", cfgPath, "-input", "-", "-outdir", outDir},
		strings.NewReader(cliRecordsCSV), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("settle from stdin exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
}

// TestValidateAllValidOrdersExitsZero is the contrast case: the validate
// subcommand on the very same inputs.
func TestValidateAllValidOrdersExitsZero(t *testing.T) {
	cfgPath, recPath, _ := cliFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"validate", "-config", cfgPath, "-input", recPath},
		strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("validate exit code = %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "订单 2 (有效 2 / 异常 0)") {
		t.Errorf("validate summary line missing from stdout:\n%s", stdout.String())
	}
}
