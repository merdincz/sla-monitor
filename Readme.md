# SLA Monitor

SLA Monitor is a CLI tool written in Go for monitoring Service Level Agreements (SLAs) such as uptime, latency, and error rate for a given target endpoint.

## Features

- **Uptime Monitoring:** Calculates the percentage of successful requests.
- **Latency Monitoring:** Reports latency percentiles (p50, p95, p99, etc.).
- **Error Rate Monitoring:** Tracks the number of failed requests.
- **Configurable:** Set parameters using a YAML configuration file or CLI flags.

## Getting Started

1. **Build the project:**
   ```bash
   make build
