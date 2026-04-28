package monitor

import (
	"testing"
	"time"

	"sla-monitor/internal/config"
)

func TestReportDataCalculatesConfiguredMetrics(t *testing.T) {
	mon := NewMonitor(&config.Config{
		SLAMetrics:         []string{"uptime", "error_rate", "latency"},
		LatencyPercentiles: []int{50, 95},
	})
	mon.startTime = time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	mon.endTime = time.Date(2026, 4, 28, 10, 1, 0, 0, time.UTC)
	mon.TotalRequests = 4
	mon.SuccessRequests = 3
	mon.failedRequests = 1
	mon.latencies = []time.Duration{
		40 * time.Millisecond,
		10 * time.Millisecond,
		30 * time.Millisecond,
		20 * time.Millisecond,
	}

	data := mon.ReportData()

	if data.TotalRequests != 4 || data.SuccessfulRequests != 3 || data.FailedRequests != 1 {
		t.Fatalf("unexpected counts: %+v", data)
	}
	if data.Uptime != 75 {
		t.Fatalf("expected uptime 75, got %v", data.Uptime)
	}
	if data.ErrorRate != 25 {
		t.Fatalf("expected error rate 25, got %v", data.ErrorRate)
	}
	if len(data.LatencyMetrics) != 2 {
		t.Fatalf("expected 2 latency metrics, got %d", len(data.LatencyMetrics))
	}
	if data.LatencyMetrics[0].Percentile != 50 || data.LatencyMetrics[0].Value != 30*time.Millisecond {
		t.Fatalf("unexpected p50 metric: %+v", data.LatencyMetrics[0])
	}
	if data.LatencyMetrics[1].Percentile != 95 || data.LatencyMetrics[1].Value != 40*time.Millisecond {
		t.Fatalf("unexpected p95 metric: %+v", data.LatencyMetrics[1])
	}
}

func TestReportDataOmitsLatencyWhenNotConfigured(t *testing.T) {
	mon := NewMonitor(&config.Config{
		SLAMetrics:         []string{"uptime"},
		LatencyPercentiles: []int{50},
	})
	mon.TotalRequests = 1
	mon.SuccessRequests = 1
	mon.latencies = []time.Duration{10 * time.Millisecond}

	data := mon.ReportData()

	if len(data.LatencyMetrics) != 0 {
		t.Fatalf("expected no latency metrics, got %+v", data.LatencyMetrics)
	}
}
