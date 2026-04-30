package monitor

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"sla-monitor/internal/config"
	"sla-monitor/internal/report"
)

type Monitor struct {
	cfg    *config.Config
	client *http.Client
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Metrics
	TotalRequests   int
	SuccessRequests int
	failedRequests  int
	startTime       time.Time
	endTime         time.Time
	latencies       []time.Duration
	mu              sync.Mutex
	reporters       []report.Reporter
}

func NewMonitor(cfg *config.Config) *Monitor {
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = "."
	}
	outputName := cfg.OutputName
	if outputName == "" {
		outputName = "sla-report"
	}

	return &Monitor{
		cfg:       cfg,
		client:    &http.Client{},
		stopCh:    make(chan struct{}),
		reporters: report.NewReporters(cfg.Output, outputDir, outputName, os.Stdout),
	}
}

func (m *Monitor) Start() {
	m.startTime = time.Now()
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	fmt.Printf("Monitoring going to start based on your frequency...\n")
	fmt.Printf("Waiting for close signal... (Command/Ctrl + C)\n")
	for {
		select {
		case <-ticker.C:
			m.wg.Add(m.cfg.Concurrency)
			for i := 0; i < m.cfg.Concurrency; i++ {
				go m.doRequest()
			}
			m.wg.Wait()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Monitor) doRequest() {
	defer m.wg.Done()

	start := time.Now()
	m.client.Timeout = 30 * time.Second
	resp, err := m.client.Get(m.cfg.Target)
	duration := time.Since(start)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	m.latencies = append(m.latencies, duration)

	if err != nil || resp.StatusCode >= 400 {
		m.failedRequests++
	} else {
		m.SuccessRequests++
	}

	if resp != nil {
		resp.Body.Close()
	}
}

func (m *Monitor) Stop() {
	m.endTime = time.Now()
	close(m.stopCh)
}

func (m *Monitor) Report() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := m.reportData()
	for _, reporter := range m.reporters {
		if err := reporter.Report(data); err != nil {
			fmt.Fprintf(os.Stderr, "report generation failed: %v\n", err)
		}
	}
}

func (m *Monitor) reportData() report.ReportData {
	data := report.ReportData{
		StartTime:          m.startTime,
		EndTime:            m.endTime,
		TotalRequests:      m.TotalRequests,
		SuccessfulRequests: m.SuccessRequests,
		FailedRequests:     m.failedRequests,
	}

	if m.TotalRequests > 0 {
		if contains(m.cfg.SLAMetrics, "uptime") {
			uptime := float64(m.SuccessRequests) / float64(m.TotalRequests) * 100
			data.Uptime = &uptime
		}
		if contains(m.cfg.SLAMetrics, "error_rate") {
			errorRate := float64(m.failedRequests) / float64(m.TotalRequests) * 100
			data.ErrorRate = &errorRate
		}
	}

	if contains(m.cfg.SLAMetrics, "latency") && len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		for _, perc := range m.cfg.LatencyPercentiles {
			index := (perc * len(sorted)) / 100
			if index >= len(sorted) {
				index = len(sorted) - 1
			}
			data.Latency = append(data.Latency, report.LatencyMetric{
				Percentile: perc,
				Duration:   sorted[index],
			})
		}
	}

	return data
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
