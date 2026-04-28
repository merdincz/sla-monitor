package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"sla-monitor/internal/monitor"
)

func sampleData() monitor.ReportData {
	return monitor.ReportData{
		StartTime:          time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		EndTime:            time.Date(2026, 4, 28, 10, 1, 0, 0, time.UTC),
		TotalRequests:      4,
		SuccessfulRequests: 3,
		FailedRequests:     1,
		SLAMetrics:         []string{"uptime", "error_rate", "latency"},
		Uptime:             75,
		ErrorRate:          25,
		LatencyMetrics: []monitor.LatencyMetric{
			{Percentile: 50, Value: 30 * time.Millisecond},
			{Percentile: 95, Value: 40 * time.Millisecond},
		},
	}
}

func TestRenderTextPreservesReportFormat(t *testing.T) {
	var buf bytes.Buffer

	if err := RenderText(&buf, sampleData()); err != nil {
		t.Fatalf("RenderText failed: %v", err)
	}

	want := "SLA Report (2026-04-28 10:00:00 to 2026-04-28 10:01:00)\n" +
		"--------------------------------------------------------\n" +
		"Total Requests: 4\n" +
		"Successful: 3\n" +
		"Failed: 1\n" +
		"Uptime: 75.00% (3/4 successful requests)\n" +
		"Error Rate: 25.00% (1/4 failed requests)\n" +
		"Latency Metrics\n" +
		"\tp50: 30ms\n" +
		"\tp95: 40ms\n"
	if buf.String() != want {
		t.Fatalf("unexpected text report:\n%s", buf.String())
	}
}

func TestRenderHTMLIncludesConfiguredSectionsAndEscapesValues(t *testing.T) {
	data := sampleData()
	data.SLAMetrics = []string{"uptime", "latency", "<script>alert(1)</script>"}

	var buf bytes.Buffer
	if err := RenderHTML(&buf, data); err != nil {
		t.Fatalf("RenderHTML failed: %v", err)
	}

	html := buf.String()
	for _, want := range []string{
		"<!doctype html>",
		"<h1>SLA Report</h1>",
		"2026-04-28 10:00:00 to 2026-04-28 10:01:00",
		"Total Requests",
		"Successful",
		"Failed",
		"Uptime",
		"Latency Metrics",
		"p50",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected HTML to contain %q:\n%s", want, html)
		}
	}
	if strings.Contains(html, "Error Rate</h2>") {
		t.Fatalf("did not expect unconfigured error rate section:\n%s", html)
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected dynamic values to be escaped:\n%s", html)
	}
}
