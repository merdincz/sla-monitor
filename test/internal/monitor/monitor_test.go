package monitor_test

import (
	"bytes"
	"encoding/csv"
	"math"
	"net/http"
	"net/http/httptest"
	"sla-monitor/internal/config"
	"sla-monitor/internal/monitor"
	"sla-monitor/internal/report"
	"strings"
	"testing"
	"time"
)

func TestMonitor_SuccessScenarios(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Target:             server.URL,
		Concurrency:        2,
		Interval:           50 * time.Millisecond,
		SLAMetrics:         []string{"uptime"},
		LatencyPercentiles: []int{50},
	}

	mon := monitor.NewMonitor(cfg)

	done := make(chan struct{})
	go func() {
		mon.Start()
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	mon.Stop()
	<-done
	if mon.TotalRequests == 0 {
		t.Error("expected some requests, got 0")
	}
	if mon.SuccessRequests != mon.TotalRequests {
		t.Error("expected all requests to succeed")
	}
}

func TestReportSnapshotCalculations(t *testing.T) {
	start := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	metrics := []string{"uptime", "error_rate", "latency"}
	percentiles := []int{50, 95}
	latencies := []time.Duration{40 * time.Millisecond, 10 * time.Millisecond, 30 * time.Millisecond, 20 * time.Millisecond}

	snapshot := report.NewSnapshot(report.BuilderInput{
		Target:             "https://example.com",
		Concurrency:        3,
		Interval:           time.Second,
		SLAMetrics:         metrics,
		LatencyPercentiles: percentiles,
		StartTime:          start,
		EndTime:            end,
		TotalRequests:      3,
		SuccessRequests:    2,
		FailedRequests:     1,
		Latencies:          latencies,
	})

	metrics[0] = "changed"
	percentiles[0] = 10

	if math.Abs(snapshot.UptimePercent-66.6666666667) > 0.0001 {
		t.Fatalf("unexpected uptime percentage: %f", snapshot.UptimePercent)
	}
	if math.Abs(snapshot.ErrorRatePercent-33.3333333333) > 0.0001 {
		t.Fatalf("unexpected error rate percentage: %f", snapshot.ErrorRatePercent)
	}
	if !snapshot.HasMetric("uptime") {
		t.Fatal("expected snapshot to keep copied SLA metrics")
	}
	if len(snapshot.Latencies) != 2 {
		t.Fatalf("expected 2 latency metrics, got %d", len(snapshot.Latencies))
	}
	if snapshot.Latencies[0].Percentile != 50 || snapshot.Latencies[0].Value != 30*time.Millisecond {
		t.Fatalf("unexpected p50 latency metric: %+v", snapshot.Latencies[0])
	}
	if snapshot.Latencies[1].Percentile != 95 || snapshot.Latencies[1].Value != 40*time.Millisecond {
		t.Fatalf("unexpected p95 latency metric: %+v", snapshot.Latencies[1])
	}
}

func TestRenderTextPreservesExistingShape(t *testing.T) {
	var out bytes.Buffer

	err := report.RenderText(&out, sampleSnapshot())
	if err != nil {
		t.Fatalf("RenderText failed: %v", err)
	}

	want := "SLA Report (2026-05-02 10:00:00 to 2026-05-02 10:01:00)\n" +
		"--------------------------------------------------------\n" +
		"Total Requests: 3\n" +
		"Successful: 2\n" +
		"Failed: 1\n" +
		"Uptime: 66.67% (2/3 successful requests)\n" +
		"Error Rate: 33.33% (1/3 failed requests)\n" +
		"Latency Metrics\n" +
		"\tp50: 20ms\n"

	if out.String() != want {
		t.Fatalf("unexpected text output:\n%s", out.String())
	}
}

func TestRenderCSVIsParseable(t *testing.T) {
	var out bytes.Buffer

	err := report.RenderCSV(&out, sampleSnapshot())
	if err != nil {
		t.Fatalf("RenderCSV failed: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv output is not parseable: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected csv header and rows, got %d records", len(records))
	}
	if records[0][0] != "metric" || records[0][1] != "value" {
		t.Fatalf("unexpected csv header: %v", records[0])
	}
}

func TestRenderMarkdownEscapesTableValues(t *testing.T) {
	var out bytes.Buffer
	snapshot := sampleSnapshot()
	snapshot.Target = "https://example.com/a|b\nnext"

	err := report.RenderMarkdown(&out, snapshot)
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}

	if !strings.Contains(out.String(), "https://example.com/a\\|b<br>next") {
		t.Fatalf("markdown target was not escaped:\n%s", out.String())
	}
}

func TestRenderHTMLEscapesAndUsesInlineAssets(t *testing.T) {
	var out bytes.Buffer
	snapshot := sampleSnapshot()
	snapshot.Target = `<script>alert("x")</script>`

	err := report.RenderHTML(&out, snapshot)
	if err != nil {
		t.Fatalf("RenderHTML failed: %v", err)
	}

	html := out.String()
	if strings.Contains(html, snapshot.Target) {
		t.Fatalf("html output contains unescaped target:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("html output does not contain escaped target:\n%s", html)
	}
	if strings.Contains(html, "<script") || strings.Contains(html, " src=") || strings.Contains(html, " href=") {
		t.Fatalf("html output should be self-contained without scripts or external assets:\n%s", html)
	}
}

func TestRenderPDFSmoke(t *testing.T) {
	var out bytes.Buffer

	err := report.RenderPDF(&out, sampleSnapshot())
	if err != nil {
		t.Fatalf("RenderPDF failed: %v", err)
	}

	pdf := out.String()
	if !strings.HasPrefix(pdf, "%PDF-1.4\n") {
		t.Fatalf("pdf output has unexpected header")
	}
	if !strings.Contains(pdf, "%%EOF") {
		t.Fatalf("pdf output is missing EOF marker")
	}
	if !strings.Contains(pdf, "SLA Report") || !strings.Contains(pdf, "Uptime: 66.67%") {
		t.Fatalf("pdf output is missing report content")
	}
}

func sampleSnapshot() report.Snapshot {
	start := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	return report.NewSnapshot(report.BuilderInput{
		Target:             "https://example.com",
		Concurrency:        2,
		Interval:           time.Second,
		SLAMetrics:         []string{"uptime", "error_rate", "latency"},
		LatencyPercentiles: []int{50},
		StartTime:          start,
		EndTime:            start.Add(time.Minute),
		TotalRequests:      3,
		SuccessRequests:    2,
		FailedRequests:     1,
		Latencies:          []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond},
	})
}
