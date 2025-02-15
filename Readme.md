# SLA Monitor

SLA Monitor is a CLI tool written in Go for monitoring Service Level Agreements (SLAs) such as uptime, latency, and error rate for a given target endpoint.

## Features

- **Uptime Monitoring:** Calculates the percentage of successful requests.
- **Latency Monitoring:** Reports latency percentiles (p50, p95, p99, etc.).
- **Error Rate Monitoring:** Tracks the number of failed requests.
- **Configurable:** Set parameters using a YAML configuration file or CLI flags.
- **Concurrent Requests:** Supports multiple concurrent requests for load testing.


## Configuration Example
You can use either with a yaml file, or pass all arguments to cli.

# Using yaml file

```yaml
target: "https://postman-echo.com/get"
concurrency: 10
interval: "1s"
sla_metrics:
  - uptime
  - latency
  - error_rate
latency_percentiles: [50, 95, 99]
```
```bash
sla-monitor --config config.yaml
```

# Using arguments
```bash
sla-monitor \
  --target https://api.example.com \
  --concurrency 10 \
  --interval 5s \
  --sla_metrics uptime,latency,error_rate \
  --latency_percentiles 50,95,99
```
## Available Options
```bash
--config: Path to config file (default: "configs/config.yaml")
--target: Target endpoint URL
--concurrency: Number of concurrent requests
--interval: Interval between requests (e.g., "5s", "1m")
--sla_metrics: Metrics to monitor (uptime, latency, error_rate)
--latency_percentiles: Latency percentiles to report
```

## Example Output
```bash
Target: https://api.example.com
Concurrency: 10
Interval: 1s

SLA Report (2024-03-15 10:00:00 to 2024-03-15 10:01:00)
--------------------------------------------------------
Total Requests: 600
Successful: 598
Failed: 2
Uptime: 99.67% (598/600 successful requests)
Error Rate: 0.33% (2/600 failed requests)
Latency Metrics
    p50: 145ms
    p95: 250ms
    p99: 350ms
```