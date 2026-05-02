package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Format string

const (
	FormatText Format = "text"
	FormatCSV  Format = "csv"
	FormatMD   Format = "md"
	FormatHTML Format = "html"
	FormatPDF  Format = "pdf"
)

type Snapshot struct {
	Target             string
	Concurrency        int
	Interval           time.Duration
	SLAMetrics         []string
	LatencyPercentiles []int
	StartTime          time.Time
	EndTime            time.Time
	TotalRequests      int
	SuccessRequests    int
	FailedRequests     int
	UptimePercent      float64
	ErrorRatePercent   float64
	Latencies          []LatencyMetric
}

type LatencyMetric struct {
	Percentile int
	Value      time.Duration
}

type BuilderInput struct {
	Target             string
	Concurrency        int
	Interval           time.Duration
	SLAMetrics         []string
	LatencyPercentiles []int
	StartTime          time.Time
	EndTime            time.Time
	TotalRequests      int
	SuccessRequests    int
	FailedRequests     int
	Latencies          []time.Duration
}

func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))
	switch format {
	case FormatText, FormatCSV, FormatMD, FormatHTML, FormatPDF:
		return format, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (valid: text, csv, md, html, pdf)", value)
	}
}

func NewSnapshot(input BuilderInput) Snapshot {
	return Snapshot{
		Target:             input.Target,
		Concurrency:        input.Concurrency,
		Interval:           input.Interval,
		SLAMetrics:         cloneStrings(input.SLAMetrics),
		LatencyPercentiles: cloneInts(input.LatencyPercentiles),
		StartTime:          input.StartTime,
		EndTime:            input.EndTime,
		TotalRequests:      input.TotalRequests,
		SuccessRequests:    input.SuccessRequests,
		FailedRequests:     input.FailedRequests,
		UptimePercent:      UptimePercent(input.SuccessRequests, input.TotalRequests),
		ErrorRatePercent:   ErrorRatePercent(input.FailedRequests, input.TotalRequests),
		Latencies:          CalculateLatencyPercentiles(input.Latencies, input.LatencyPercentiles),
	}
}

func UptimePercent(successRequests, totalRequests int) float64 {
	if totalRequests <= 0 {
		return 0
	}
	return float64(successRequests) / float64(totalRequests) * 100
}

func ErrorRatePercent(failedRequests, totalRequests int) float64 {
	if totalRequests <= 0 {
		return 0
	}
	return float64(failedRequests) / float64(totalRequests) * 100
}

func CalculateLatencyPercentiles(latencies []time.Duration, percentiles []int) []LatencyMetric {
	if len(latencies) == 0 || len(percentiles) == 0 {
		return nil
	}

	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	sort.Slice(sortedLatencies, func(i, j int) bool { return sortedLatencies[i] < sortedLatencies[j] })

	metrics := make([]LatencyMetric, 0, len(percentiles))
	for _, percentile := range percentiles {
		index := (percentile * len(sortedLatencies)) / 100
		if index >= len(sortedLatencies) {
			index = len(sortedLatencies) - 1
		}
		if index < 0 {
			index = 0
		}
		metrics = append(metrics, LatencyMetric{
			Percentile: percentile,
			Value:      sortedLatencies[index],
		})
	}

	return metrics
}

func Render(w io.Writer, format Format, snapshot Snapshot) error {
	switch format {
	case FormatText:
		return RenderText(w, snapshot)
	case FormatCSV:
		return RenderCSV(w, snapshot)
	case FormatMD:
		return RenderMarkdown(w, snapshot)
	case FormatHTML:
		return RenderHTML(w, snapshot)
	case FormatPDF:
		return RenderPDF(w, snapshot)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func RenderText(w io.Writer, snapshot Snapshot) error {
	if _, err := fmt.Fprintf(w, "SLA Report (%s to %s)\n", formatTime(snapshot.StartTime), formatTime(snapshot.EndTime)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "--------------------------------------------------------"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total Requests: %d\n", snapshot.TotalRequests); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Successful: %d\n", snapshot.SuccessRequests); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Failed: %d\n", snapshot.FailedRequests); err != nil {
		return err
	}

	if snapshot.HasMetric("uptime") && snapshot.TotalRequests > 0 {
		if _, err := fmt.Fprintf(w, "Uptime: %.2f%% (%d/%d successful requests)\n", snapshot.UptimePercent, snapshot.SuccessRequests, snapshot.TotalRequests); err != nil {
			return err
		}
	}

	if snapshot.HasMetric("error_rate") && snapshot.TotalRequests > 0 {
		if _, err := fmt.Fprintf(w, "Error Rate: %.2f%% (%d/%d failed requests)\n", snapshot.ErrorRatePercent, snapshot.FailedRequests, snapshot.TotalRequests); err != nil {
			return err
		}
	}

	if snapshot.HasMetric("latency") && len(snapshot.Latencies) > 0 {
		if _, err := fmt.Fprintln(w, "Latency Metrics"); err != nil {
			return err
		}
		for _, metric := range snapshot.Latencies {
			if _, err := fmt.Fprintf(w, "\tp%d: %v\n", metric.Percentile, metric.Value); err != nil {
				return err
			}
		}
	}

	return nil
}

func RenderCSV(w io.Writer, snapshot Snapshot) error {
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write([]string{"metric", "value"}); err != nil {
		return err
	}
	for _, row := range snapshotRows(snapshot) {
		if err := csvWriter.Write([]string{row.Name, row.Value}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func RenderMarkdown(w io.Writer, snapshot Snapshot) error {
	if _, err := fmt.Fprintln(w, "| Metric | Value |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- |"); err != nil {
		return err
	}
	for _, row := range snapshotRows(snapshot) {
		if _, err := fmt.Fprintf(w, "| %s | %s |\n", escapeMarkdown(row.Name), escapeMarkdown(row.Value)); err != nil {
			return err
		}
	}
	return nil
}

func RenderHTML(w io.Writer, snapshot Snapshot) error {
	data := struct {
		Snapshot Snapshot
		Rows     []row
	}{
		Snapshot: snapshot,
		Rows:     snapshotRows(snapshot),
	}

	return htmlReportTemplate.Execute(w, data)
}

func RenderPDF(w io.Writer, snapshot Snapshot) error {
	var lines []string
	lines = append(lines, fmt.Sprintf("SLA Report (%s to %s)", formatTime(snapshot.StartTime), formatTime(snapshot.EndTime)))
	for _, row := range snapshotRows(snapshot) {
		lines = append(lines, fmt.Sprintf("%s: %s", row.Name, row.Value))
	}
	return writeSimplePDF(w, lines)
}

func (s Snapshot) HasMetric(metric string) bool {
	for _, configured := range s.SLAMetrics {
		if configured == metric {
			return true
		}
	}
	return false
}

type row struct {
	Name  string
	Value string
}

func snapshotRows(snapshot Snapshot) []row {
	rows := []row{
		{Name: "Target", Value: snapshot.Target},
		{Name: "Start Time", Value: formatTime(snapshot.StartTime)},
		{Name: "End Time", Value: formatTime(snapshot.EndTime)},
		{Name: "Concurrency", Value: strconv.Itoa(snapshot.Concurrency)},
		{Name: "Interval", Value: snapshot.Interval.String()},
		{Name: "Total Requests", Value: strconv.Itoa(snapshot.TotalRequests)},
		{Name: "Successful", Value: strconv.Itoa(snapshot.SuccessRequests)},
		{Name: "Failed", Value: strconv.Itoa(snapshot.FailedRequests)},
	}

	if snapshot.HasMetric("uptime") && snapshot.TotalRequests > 0 {
		rows = append(rows, row{
			Name:  "Uptime",
			Value: fmt.Sprintf("%.2f%% (%d/%d successful requests)", snapshot.UptimePercent, snapshot.SuccessRequests, snapshot.TotalRequests),
		})
	}

	if snapshot.HasMetric("error_rate") && snapshot.TotalRequests > 0 {
		rows = append(rows, row{
			Name:  "Error Rate",
			Value: fmt.Sprintf("%.2f%% (%d/%d failed requests)", snapshot.ErrorRatePercent, snapshot.FailedRequests, snapshot.TotalRequests),
		})
	}

	if snapshot.HasMetric("latency") {
		for _, metric := range snapshot.Latencies {
			rows = append(rows, row{
				Name:  fmt.Sprintf("Latency p%d", metric.Percentile),
				Value: metric.Value.String(),
			})
		}
	}

	return rows
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "\r", "<br>")
	return value
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneInts(values []int) []int {
	if values == nil {
		return nil
	}
	cloned := make([]int, len(values))
	copy(cloned, values)
	return cloned
}

var htmlReportTemplate = template.Must(template.New("html-report").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>SLA Report</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f5f7fb;
      --panel: #ffffff;
      --ink: #172033;
      --muted: #68748a;
      --line: #dde3ee;
      --accent: #0f766e;
      --accent-soft: #d9f3ef;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: var(--bg);
      color: var(--ink);
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      line-height: 1.5;
    }
    main {
      width: min(980px, calc(100% - 32px));
      margin: 40px auto;
    }
    .report {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: 0 18px 45px rgba(23, 32, 51, 0.08);
      overflow: hidden;
    }
    header {
      padding: 28px 32px;
      border-bottom: 1px solid var(--line);
      background: linear-gradient(135deg, #ffffff 0%, #eef7f5 100%);
    }
    h1 {
      margin: 0 0 8px;
      font-size: clamp(1.8rem, 3vw, 2.6rem);
      line-height: 1.1;
      letter-spacing: 0;
    }
    .subtitle {
      color: var(--muted);
      margin: 0;
      overflow-wrap: anywhere;
    }
    table {
      width: 100%;
      border-collapse: collapse;
    }
    th, td {
      padding: 16px 20px;
      text-align: left;
      border-bottom: 1px solid var(--line);
      vertical-align: top;
      overflow-wrap: anywhere;
    }
    th {
      width: 34%;
      color: var(--muted);
      font-size: 0.78rem;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      background: #fbfcfe;
    }
    td {
      font-weight: 650;
    }
    tr:last-child th, tr:last-child td { border-bottom: 0; }
    .badge {
      display: inline-flex;
      align-items: center;
      min-height: 30px;
      padding: 4px 10px;
      border-radius: 999px;
      background: var(--accent-soft);
      color: var(--accent);
      font-weight: 750;
    }
    @media (max-width: 640px) {
      main { width: min(100% - 20px, 980px); margin: 20px auto; }
      header { padding: 22px 20px; }
      th, td { display: block; width: 100%; padding: 12px 16px; }
      th { border-bottom: 0; padding-bottom: 4px; }
      td { padding-top: 4px; }
    }
  </style>
</head>
<body>
  <main>
    <section class="report" aria-labelledby="report-title">
      <header>
        <h1 id="report-title">SLA Report</h1>
        <p class="subtitle">{{.Snapshot.StartTime.Format "2006-01-02 15:04:05"}} to {{.Snapshot.EndTime.Format "2006-01-02 15:04:05"}}</p>
      </header>
      <table>
        <tbody>
        {{range .Rows}}
          <tr>
            <th scope="row">{{.Name}}</th>
            <td>{{if eq .Name "Uptime"}}<span class="badge">{{.Value}}</span>{{else}}{{.Value}}{{end}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </section>
  </main>
</body>
</html>
`))

func writeSimplePDF(w io.Writer, lines []string) error {
	var content bytes.Buffer
	content.WriteString("BT\n/F1 12 Tf\n50 780 Td\n")
	for index, line := range lines {
		if index > 0 {
			content.WriteString("0 -18 Td\n")
		}
		content.WriteString("(")
		content.WriteString(escapePDFText(line))
		content.WriteString(") Tj\n")
	}
	content.WriteString("ET\n")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, len(objects)+1)
	offsets = append(offsets, 0)
	for index, object := range objects {
		offsets = append(offsets, pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}

	xrefOffset := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n", len(objects)+1)
	pdf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)

	_, err := w.Write(pdf.Bytes())
	return err
}

func escapePDFText(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	value = strings.ReplaceAll(value, ")", "\\)")
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}
