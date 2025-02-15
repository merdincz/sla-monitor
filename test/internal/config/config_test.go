package config_test

import (
	"os"
	"testing"
	"time"

	"sla-monitor/internal/config"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write test config
	configData := []byte(`
target: "http://test.com"
concurrency: 5
interval: "2s"
sla_metrics:
  - uptime
  - latency
latency_percentiles: [50, 90]
`)
	if _, err := tmpfile.Write(configData); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Test loading config
	cfg, err := config.LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify loaded config
	if cfg.Target != "http://test.com" {
		t.Errorf("expected target 'http://test.com', got %s", cfg.Target)
	}
	if cfg.Concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", cfg.Concurrency)
	}
	if cfg.Interval != 2*time.Second {
		t.Errorf("expected interval 2s, got %v", cfg.Interval)
	}
}
