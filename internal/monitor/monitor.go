package monitor

import (
	"fmt"
	"net/http"
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

func (m *Monitor) Report() {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Printf("SLA Report (%s to %s)\n", m.startTime.Format("2006-01-02 15:04:05"), m.endTime.Format("2006-01-02 15:04:05"))
	fmt.Println("--------------------------------------------------------")
	fmt.Printf("Total Requests: %d\n", m.TotalRequests)
	fmt.Printf("Successful: %d\n", m.SuccessRequests)
	fmt.Printf("Failed: %d\n", m.failedRequests)

	// Uptime
	if contains(m.cfg.SLAMetrics, "uptime") && m.TotalRequests > 0 {
		uptime := float64(m.SuccessRequests) / float64(m.TotalRequests) * 100
		fmt.Printf("Uptime: %.2f%% (%d/%d successful requests)\n", uptime, m.SuccessRequests, m.TotalRequests)
	}

	// Error rate
	if contains(m.cfg.SLAMetrics, "error_rate") && m.TotalRequests > 0 {
		errorRate := float64(m.failedRequests) / float64(m.TotalRequests) * 100
		fmt.Printf("Error Rate: %.2f%% (%d/%d failed requests)\n", errorRate, m.failedRequests, m.TotalRequests)
	}

	// Latency
	if contains(m.cfg.SLAMetrics, "latency") && len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		fmt.Println("Latency Metrics")
		for _, perc := range m.cfg.LatencyPercentiles {
			index := (perc * len(sorted)) / 100
			if index >= len(sorted) {
				index = len(sorted) - 1
			}
			fmt.Printf("\tp%d: %v\n", perc, sorted[index])
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
