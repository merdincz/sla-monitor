package monitor_test

import (
	"net/http"
	"net/http/httptest"
	"sla-monitor/internal/config"
	"sla-monitor/internal/monitor"
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
