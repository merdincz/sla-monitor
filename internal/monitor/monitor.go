package monitor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"sla-monitor/internal/config"
	"sla-monitor/internal/report"
)

type Monitor struct {
	cfg       *config.Config
	client    *http.Client
	stopCh    chan struct{}
	wg        sync.WaitGroup
	logWriter io.Writer

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
		cfg:       cfg,
		client:    &http.Client{Timeout: 30 * time.Second},
		stopCh:    make(chan struct{}),
		logWriter: os.Stdout,
	}
}

func (m *Monitor) SetLogWriter(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w == nil {
		m.logWriter = io.Discard
		return
	}
	m.logWriter = w
}

func (m *Monitor) Start() {
	m.startTime = time.Now()
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	fmt.Fprintln(m.logWriter, "Monitoring going to start based on your frequency...")
	fmt.Fprintln(m.logWriter, "Waiting for close signal... (Command/Ctrl + C)")
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
	_ = report.RenderText(os.Stdout, m.Snapshot())
}

func (m *Monitor) Snapshot() report.Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	latencies := make([]time.Duration, len(m.latencies))
	copy(latencies, m.latencies)

	return report.NewSnapshot(report.BuilderInput{
		Target:             m.cfg.Target,
		Concurrency:        m.cfg.Concurrency,
		Interval:           m.cfg.Interval,
		SLAMetrics:         m.cfg.SLAMetrics,
		LatencyPercentiles: m.cfg.LatencyPercentiles,
		StartTime:          m.startTime,
		EndTime:            m.endTime,
		TotalRequests:      m.TotalRequests,
		SuccessRequests:    m.SuccessRequests,
		FailedRequests:     m.failedRequests,
		Latencies:          latencies,
	})
}

func (m *Monitor) FailedRequests() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failedRequests
}

func (m *Monitor) LogWriter() io.Writer {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logWriter == nil {
		return io.Discard
	}
	return m.logWriter
}
