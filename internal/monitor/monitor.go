package monitor

import (
	"fmt"
	"net/http"
	"strings"
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
}

func NewMonitor(cfg *config.Config) *Monitor {
	return &Monitor{
		cfg:    cfg,
		client: &http.Client{},
		stopCh: make(chan struct{}),
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

func (m *Monitor) BuildReportData() report.ReportData {
	m.mu.Lock()
	defer m.mu.Unlock()

	latCopy := make([]time.Duration, len(m.latencies))
	copy(latCopy, m.latencies)

	return report.BuildReportData(
		m.startTime,
		m.endTime,
		m.TotalRequests,
		m.SuccessRequests,
		m.failedRequests,
		latCopy,
		m.cfg.SLAMetrics,
		m.cfg.LatencyPercentiles,
	)
}

func (m *Monitor) Report() {
	data := m.BuildReportData()

	fmt.Printf("SLA Report (%s to %s)\n", data.StartTime, data.EndTime)
	fmt.Println("--------------------------------------------------------")
	for _, row := range data.Rows {
		if strings.HasPrefix(row.Name, "p") {
			continue
		}
		fmt.Printf("%s: %s\n", row.Name, row.Value)
	}

	if contains(m.cfg.SLAMetrics, "latency") {
		hasLatency := false
		for _, row := range data.Rows {
			if strings.HasPrefix(row.Name, "p") {
				if !hasLatency {
					fmt.Println("Latency Metrics")
					hasLatency = true
				}
				fmt.Printf("\t%s: %s\n", row.Name, row.Value)
			}
		}
	}
}

func (m *Monitor) ReportV2(format, outputFile string) error {
	manager := report.NewOutputManager()
	return manager.Emit(format, outputFile, m.BuildReportData())
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
