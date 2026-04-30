package report_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sla-monitor/internal/report"
)

func sampleData() report.ReportData {
	uptime := 90.0
	errorRate := 10.0
	return report.ReportData{
		StartTime:          time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC),
		EndTime:            time.Date(2026, 4, 30, 8, 5, 0, 0, time.UTC),
		TotalRequests:      10,
		SuccessfulRequests: 9,
		FailedRequests:     1,
		Uptime:             &uptime,
		ErrorRate:          &errorRate,
		Latency: []report.LatencyMetric{
			{Percentile: 50, Duration: 20 * time.Millisecond},
			{Percentile: 95, Duration: 40 * time.Millisecond},
		},
	}
}

func TestNewReporters_AlwaysIncludesStdoutAndAdditionalOutputs(t *testing.T) {
	reporters := report.NewReporters([]string{"csv", "html", "md", "pdf"}, t.TempDir(), "report", &bytes.Buffer{})
	if len(reporters) != 5 {
		t.Fatalf("expected 5 reporters (stdout + 4), got %d", len(reporters))
	}
}

func TestReporters_GenerateFiles(t *testing.T) {
	dir := t.TempDir()
	reporters := report.NewReporters([]string{"csv", "html", "md", "pdf"}, dir, "report", &bytes.Buffer{})
	for _, rep := range reporters {
		if err := rep.Report(sampleData()); err != nil {
			t.Fatalf("unexpected report error: %v", err)
		}
	}

	for _, ext := range []string{"csv", "html", "md", "pdf"} {
		path := filepath.Join(dir, "report."+ext)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}

	content, err := os.ReadFile(filepath.Join(dir, "report.csv"))
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if !strings.Contains(string(content), "total_requests") {
		t.Fatalf("expected csv to contain metrics, got: %s", string(content))
	}
}

func TestReporterFailure_DoesNotBlockOthers(t *testing.T) {
	dir := t.TempDir()
	blockedPath := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockedPath, []byte("file not dir"), 0o644); err != nil {
		t.Fatalf("write blocked path: %v", err)
	}

	goodDir := filepath.Join(dir, "good")
	reporters := report.NewReporters([]string{"csv", "md"}, blockedPath, "report", &bytes.Buffer{})
	if err := reporters[1].Report(sampleData()); err == nil {
		t.Fatalf("expected csv reporter to fail due to invalid output dir")
	}

	goodReporters := report.NewReporters([]string{"md"}, goodDir, "report", &bytes.Buffer{})
	if err := goodReporters[1].Report(sampleData()); err != nil {
		t.Fatalf("expected markdown reporter to succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(goodDir, "report.md")); err != nil {
		t.Fatalf("expected markdown file generated: %v", err)
	}
}
