package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sla-monitor/internal/report"
)

func TestBuildReportDataAndStdoutRenderer(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	data := report.BuildReportData(
		start,
		end,
		10,
		8,
		2,
		[]time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 150 * time.Millisecond},
		[]string{"uptime", "error_rate", "latency"},
		[]int{50, 95},
	)

	if len(data.Rows) < 7 {
		t.Fatalf("expected at least 7 rows, got %d", len(data.Rows))
	}

	out, err := (report.StdoutRenderer{}).Render(data)
	if err != nil {
		t.Fatalf("render stdout: %v", err)
	}
	text := string(out)
	checks := []string{
		"SLA Report (2026-01-02 03:04:05 to 2026-01-02 03:06:05)",
		"Total Requests: 10",
		"Successful: 8",
		"Failed: 2",
		"Uptime: 80.00% (8/10 successful requests)",
		"Error Rate: 20.00% (2/10 failed requests)",
		"p50: 150ms",
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Fatalf("stdout output missing %q\noutput=%s", check, text)
		}
	}
}

func TestValidateOutputConfig(t *testing.T) {
	t.Run("invalid format", func(t *testing.T) {
		if err := report.ValidateOutputConfig("json", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("stdout with output file", func(t *testing.T) {
		if err := report.ValidateOutputConfig("stdout", "x.txt"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("non stdout missing output file", func(t *testing.T) {
		if err := report.ValidateOutputConfig("csv", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid non stdout", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "report.csv")
		if err := report.ValidateOutputConfig("csv", file); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("html extension required", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "report.txt")
		err := report.ValidateOutputConfig("html", file)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "requires --output-file extension .html,.htm") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("pdf extension required", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "report")
		err := report.ValidateOutputConfig("pdf", file)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "requires --output-file extension .pdf") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestOutputManagerWritesFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.md")
	data := report.ReportData{
		StartTime: "2026-01-02 03:04:05",
		EndTime:   "2026-01-02 03:06:05",
		Rows: []report.MetricRow{
			{Name: "Total Requests", Value: "5"},
			{Name: "Successful", Value: "5"},
		},
	}

	m := report.NewOutputManager()
	if err := m.Emit("md", file, data); err != nil {
		t.Fatalf("emit: %v", err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(content), "| Total Requests | 5 |") {
		t.Fatalf("unexpected markdown content: %s", string(content))
	}
}

func TestPDFRendererLimits(t *testing.T) {
	_, err := (report.PDFRenderer{}).Render(report.ReportData{})
	if err == nil {
		t.Fatal("expected error for empty rows")
	}
}
