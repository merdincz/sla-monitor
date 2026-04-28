package report

import (
	"fmt"
	"html/template"
	"io"

	"sla-monitor/internal/monitor"
)

func RenderText(w io.Writer, data monitor.ReportData) error {
	if _, err := io.WriteString(w, "SLA Report ("+data.StartTime.Format("2006-01-02 15:04:05")+" to "+data.EndTime.Format("2006-01-02 15:04:05")+")\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "--------------------------------------------------------\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, formatCounts(data)); err != nil {
		return err
	}
	if hasMetric(data.SLAMetrics, "uptime") && data.TotalRequests > 0 {
		if _, err := io.WriteString(w, formatUptime(data)); err != nil {
			return err
		}
	}
	if hasMetric(data.SLAMetrics, "error_rate") && data.TotalRequests > 0 {
		if _, err := io.WriteString(w, formatErrorRate(data)); err != nil {
			return err
		}
	}
	if hasMetric(data.SLAMetrics, "latency") && len(data.LatencyMetrics) > 0 {
		if _, err := io.WriteString(w, "Latency Metrics\n"); err != nil {
			return err
		}
		for _, metric := range data.LatencyMetrics {
			if _, err := io.WriteString(w, formatLatency(metric)); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatCounts(data monitor.ReportData) string {
	return fmt.Sprintf("Total Requests: %d\nSuccessful: %d\nFailed: %d\n", data.TotalRequests, data.SuccessfulRequests, data.FailedRequests)
}

func formatUptime(data monitor.ReportData) string {
	return fmt.Sprintf("Uptime: %.2f%% (%d/%d successful requests)\n", data.Uptime, data.SuccessfulRequests, data.TotalRequests)
}

func formatErrorRate(data monitor.ReportData) string {
	return fmt.Sprintf("Error Rate: %.2f%% (%d/%d failed requests)\n", data.ErrorRate, data.FailedRequests, data.TotalRequests)
}

func formatLatency(metric monitor.LatencyMetric) string {
	return fmt.Sprintf("\tp%d: %v\n", metric.Percentile, metric.Value)
}

func RenderHTML(w io.Writer, data monitor.ReportData) error {
	return htmlTemplate.Execute(w, htmlData{
		ReportData:    data,
		ShowUptime:    hasMetric(data.SLAMetrics, "uptime") && data.TotalRequests > 0,
		ShowErrorRate: hasMetric(data.SLAMetrics, "error_rate") && data.TotalRequests > 0,
		ShowLatency:   hasMetric(data.SLAMetrics, "latency") && len(data.LatencyMetrics) > 0,
	})
}

type htmlData struct {
	monitor.ReportData
	ShowUptime    bool
	ShowErrorRate bool
	ShowLatency   bool
}

var htmlTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>SLA Report</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 2rem; color: #202124; background: #fff; }
    main { max-width: 760px; }
    h1 { font-size: 1.75rem; margin-bottom: 0.25rem; }
    table { border-collapse: collapse; margin-top: 1rem; width: 100%; }
    th, td { border: 1px solid #d0d7de; padding: 0.5rem; text-align: left; }
    th { background: #f6f8fa; }
    section { margin-top: 1.5rem; }
  </style>
</head>
<body>
  <main>
    <h1>SLA Report</h1>
    <p>{{ .StartTime.Format "2006-01-02 15:04:05" }} to {{ .EndTime.Format "2006-01-02 15:04:05" }}</p>
    <section>
      <h2>Summary</h2>
      <table>
        <tbody>
          <tr><th scope="row">Total Requests</th><td>{{ .TotalRequests }}</td></tr>
          <tr><th scope="row">Successful</th><td>{{ .SuccessfulRequests }}</td></tr>
          <tr><th scope="row">Failed</th><td>{{ .FailedRequests }}</td></tr>
        </tbody>
      </table>
    </section>
    {{ if .ShowUptime }}
    <section>
      <h2>Uptime</h2>
      <p>{{ printf "%.2f" .Uptime }}% ({{ .SuccessfulRequests }}/{{ .TotalRequests }} successful requests)</p>
    </section>
    {{ end }}
    {{ if .ShowErrorRate }}
    <section>
      <h2>Error Rate</h2>
      <p>{{ printf "%.2f" .ErrorRate }}% ({{ .FailedRequests }}/{{ .TotalRequests }} failed requests)</p>
    </section>
    {{ end }}
    {{ if .ShowLatency }}
    <section>
      <h2>Latency Metrics</h2>
      <table>
        <thead><tr><th scope="col">Percentile</th><th scope="col">Latency</th></tr></thead>
        <tbody>
          {{ range .LatencyMetrics }}
          <tr><td>p{{ .Percentile }}</td><td>{{ .Value }}</td></tr>
          {{ end }}
        </tbody>
      </table>
    </section>
    {{ end }}
  </main>
</body>
</html>
`))

func hasMetric(metrics []string, metric string) bool {
	for _, item := range metrics {
		if item == metric {
			return true
		}
	}
	return false
}
