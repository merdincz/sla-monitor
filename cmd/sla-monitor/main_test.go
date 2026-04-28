package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"sla-monitor/internal/config"
	"sla-monitor/internal/monitor"

	"github.com/spf13/viper"
)

type fakeMonitor struct {
	started   bool
	stopped   bool
	startedCh chan struct{}
	data      monitor.ReportData
}

func (m *fakeMonitor) Start() {
	m.started = true
	if m.startedCh != nil {
		close(m.startedCh)
	}
}

func (m *fakeMonitor) Stop() {
	m.stopped = true
}

func (m *fakeMonitor) ReportData() monitor.ReportData {
	return m.data
}

func TestRootCommandRejectsInvalidOutputBeforeMonitoringStarts(t *testing.T) {
	viper.Reset()
	var stdout, stderr bytes.Buffer
	started := false
	cmd := newRootCommand(&stdout, &stderr, func(cfg *config.Config, status io.Writer) appMonitor {
		started = true
		return &fakeMonitor{}
	}, func() os.Signal { return nil })
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected invalid output error")
	}
	if !strings.Contains(err.Error(), "supported values are text and html") {
		t.Fatalf("unexpected error: %v", err)
	}
	if started {
		t.Fatal("monitor started for invalid output")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no report on stdout, got:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no status output on stderr, got:\n%s", stderr.String())
	}
}

func TestRootCommandDefaultOutputWritesTextReportToStdoutAndStatusToStderr(t *testing.T) {
	stdout, stderr, fake := executeCommand(t, nil)

	if !strings.Contains(stdout, "SLA Report (2026-04-28 10:00:00 to 2026-04-28 10:01:00)") {
		t.Fatalf("expected text report on stdout, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "Target:") || strings.Contains(stdout, "Shutting down monitor") {
		t.Fatalf("expected stdout to contain only report, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Target: http://example.com") || !strings.Contains(stderr, "Shutting down monitor") {
		t.Fatalf("expected status messages on stderr, got:\n%s", stderr)
	}
	if !fake.started || !fake.stopped {
		t.Fatalf("expected monitor lifecycle to run, got started=%v stopped=%v", fake.started, fake.stopped)
	}
}

func TestRootCommandOutputText(t *testing.T) {
	stdout, _, _ := executeCommand(t, []string{"--output", "text"})

	if !strings.Contains(stdout, "Total Requests: 4") {
		t.Fatalf("expected text output, got:\n%s", stdout)
	}
}

func TestRootCommandOutputHTML(t *testing.T) {
	stdout, stderr, _ := executeCommand(t, []string{"--output", "html"})

	if !strings.HasPrefix(stdout, "<!doctype html>") || !strings.Contains(stdout, "<h1>SLA Report</h1>") {
		t.Fatalf("expected clean HTML output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "Target:") || strings.Contains(stdout, "Shutting down monitor") {
		t.Fatalf("expected stdout to contain only HTML report, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Target: http://example.com") {
		t.Fatalf("expected status on stderr, got:\n%s", stderr)
	}
}

func executeCommand(t *testing.T, args []string) (string, string, *fakeMonitor) {
	t.Helper()
	viper.Reset()

	cfgFile := writeConfig(t)
	data := monitor.ReportData{
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
		},
	}
	fake := &fakeMonitor{
		startedCh: make(chan struct{}),
		data:      data,
	}

	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(&stdout, &stderr, func(cfg *config.Config, status io.Writer) appMonitor {
		if cfg.Target != "http://example.com" {
			t.Fatalf("unexpected config target: %s", cfg.Target)
		}
		return fake
	}, func() os.Signal {
		<-fake.startedCh
		return nil
	})
	cmd.SetArgs(append([]string{"--config", cfgFile}, args...))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	return stdout.String(), stderr.String(), fake
}

func writeConfig(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(`target: "http://example.com"
concurrency: 2
interval: "1s"
sla_metrics: ["uptime", "error_rate", "latency"]
latency_percentiles: [50]
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}
