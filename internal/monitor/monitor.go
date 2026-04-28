package monitor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"sla-monitor/internal/config"
)

type Monitor struct {
	cfg    *config.Config
	client *http.Client
	stopCh chan struct{}
	wg     sync.WaitGroup
	status io.Writer

	// Metrics
	TotalRequests   int
	SuccessRequests int
	failedRequests  int
	startTime       time.Time
	endTime         time.Time
	latencies       []time.Duration
	mu              sync.Mutex
}

type ReportData struct {
	StartTime          time.Time
	EndTime            time.Time
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	SLAMetrics         []string
	LatencyPercentiles []int
	Uptime             float64
	ErrorRate          float64
	LatencyMetrics     []LatencyMetric
}

type LatencyMetric struct {
	Percentile int
	Value      time.Duration
}

func NewMonitor(cfg *config.Config) *Monitor {
	return NewMonitorWithStatusWriter(cfg, os.Stderr)
}

func NewMonitorWithStatusWriter(cfg *config.Config, status io.Writer) *Monitor {
	return &Monitor{
		cfg:    cfg,
		client: &http.Client{},
		stopCh: make(chan struct{}),
		status: status,
	}
}

func (m *Monitor) Start() {
	m.startTime = time.Now()
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	fmt.Fprintln(m.status, "Monitoring going to start based on your frequency...")
	fmt.Fprintln(m.status, "Waiting for close signal... (Command/Ctrl + C)")
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
	data := m.ReportData()

	fmt.Printf("SLA Report (%s to %s)\n", data.StartTime.Format("2006-01-02 15:04:05"), data.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Println("--------------------------------------------------------")
	fmt.Printf("Total Requests: %d\n", data.TotalRequests)
	fmt.Printf("Successful: %d\n", data.SuccessfulRequests)
	fmt.Printf("Failed: %d\n", data.FailedRequests)

	if contains(data.SLAMetrics, "uptime") && data.TotalRequests > 0 {
		fmt.Printf("Uptime: %.2f%% (%d/%d successful requests)\n", data.Uptime, data.SuccessfulRequests, data.TotalRequests)
	}

	if contains(data.SLAMetrics, "error_rate") && data.TotalRequests > 0 {
		fmt.Printf("Error Rate: %.2f%% (%d/%d failed requests)\n", data.ErrorRate, data.FailedRequests, data.TotalRequests)
	}

	if contains(data.SLAMetrics, "latency") && len(data.LatencyMetrics) > 0 {
		fmt.Println("Latency Metrics")
		for _, metric := range data.LatencyMetrics {
			fmt.Printf("\tp%d: %v\n", metric.Percentile, metric.Value)
		}
	}
}

func (m *Monitor) ReportData() ReportData {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := ReportData{
		StartTime:          m.startTime,
		EndTime:            m.endTime,
		TotalRequests:      m.TotalRequests,
		SuccessfulRequests: m.SuccessRequests,
		FailedRequests:     m.failedRequests,
		SLAMetrics:         append([]string(nil), m.cfg.SLAMetrics...),
		LatencyPercentiles: append([]int(nil), m.cfg.LatencyPercentiles...),
	}

	if m.TotalRequests > 0 {
		data.Uptime = float64(m.SuccessRequests) / float64(m.TotalRequests) * 100
		data.ErrorRate = float64(m.failedRequests) / float64(m.TotalRequests) * 100
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
			data.LatencyMetrics = append(data.LatencyMetrics, LatencyMetric{
				Percentile: perc,
				Value:      sorted[index],
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
