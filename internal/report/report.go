package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

type OutputFormat string

const (
	OutputStdout OutputFormat = "stdout"
	OutputCSV    OutputFormat = "csv"
	OutputMD     OutputFormat = "md"
	OutputHTML   OutputFormat = "html"
	OutputPDF    OutputFormat = "pdf"
)

var validFormats = map[OutputFormat]struct{}{
	OutputStdout: {},
	OutputCSV:    {},
	OutputMD:     {},
	OutputHTML:   {},
	OutputPDF:    {},
}

type MetricRow struct {
	Name  string
	Value string
}

type ReportData struct {
	StartTime string
	EndTime   string
	Rows      []MetricRow
}

type Renderer interface {
	Render(data ReportData) ([]byte, error)
}

type OutputManager struct {
	renderers map[OutputFormat]Renderer
}

func NewOutputManager() *OutputManager {
	return &OutputManager{renderers: map[OutputFormat]Renderer{
		OutputStdout: StdoutRenderer{},
		OutputCSV:    CSVRenderer{},
		OutputMD:     MarkdownRenderer{},
		OutputHTML:   HTMLRenderer{},
		OutputPDF:    PDFRenderer{},
	}}
}

func IsValidFormat(raw string) bool {
	_, ok := validFormats[OutputFormat(raw)]
	return ok
}

func ValidateOutputConfig(formatRaw, outputFile string) error {
	if !IsValidFormat(formatRaw) {
		return fmt.Errorf("invalid --output value %q; allowed: stdout,csv,md,html,pdf", formatRaw)
	}

	format := OutputFormat(formatRaw)
	if format == OutputStdout {
		if strings.TrimSpace(outputFile) != "" {
			return fmt.Errorf("--output-file is only valid for non-stdout outputs")
		}
		return nil
	}

	if strings.TrimSpace(outputFile) == "" {
		return fmt.Errorf("--output-file is required when --output=%s", format)
	}

	dir := filepath.Dir(outputFile)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", dir)
		}
		return fmt.Errorf("failed to inspect output directory %s: %w", dir, err)
	}

	f, err := os.CreateTemp(dir, ".writable-check-*")
	if err != nil {
		return fmt.Errorf("output path is not writable (%s): %w", outputFile, err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("failed to close writable check file: %w", err)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("failed to clean writable check file: %w", err)
	}

	return nil
}

func (m *OutputManager) Emit(formatRaw string, outputFile string, data ReportData) error {
	format := OutputFormat(formatRaw)
	renderer, ok := m.renderers[format]
	if !ok {
		return fmt.Errorf("unsupported output format: %s", format)
	}

	content, err := renderer.Render(data)
	if err != nil {
		return err
	}

	if format == OutputStdout {
		_, err = os.Stdout.Write(content)
		return err
	}

	return writeFileAtomic(outputFile, content, 0o644)
}

type StdoutRenderer struct{}

type CSVRenderer struct{}

type MarkdownRenderer struct{}

type HTMLRenderer struct{}

type PDFRenderer struct{}

func (StdoutRenderer) Render(data ReportData) ([]byte, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("SLA Report (%s to %s)\n", data.StartTime, data.EndTime))
	b.WriteString("--------------------------------------------------------\n")
	for _, row := range data.Rows {
		b.WriteString(fmt.Sprintf("%s: %s\n", row.Name, row.Value))
	}
	return []byte(b.String()), nil
}

func (CSVRenderer) Render(data ReportData) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	if err := writer.Write([]string{"metric", "value"}); err != nil {
		return nil, err
	}
	for _, row := range data.Rows {
		if err := writer.Write([]string{row.Name, row.Value}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (MarkdownRenderer) Render(data ReportData) ([]byte, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# SLA Report\n\n%s to %s\n\n", data.StartTime, data.EndTime))
	b.WriteString("| Metric | Value |\n")
	b.WriteString("| --- | --- |\n")
	for _, row := range data.Rows {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", escapeMD(row.Name), escapeMD(row.Value)))
	}
	return []byte(b.String()), nil
}

func (HTMLRenderer) Render(data ReportData) ([]byte, error) {
	tpl := template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>SLA Report</title>
  <style>
    :root { --bg: #f5f7fb; --card: #ffffff; --ink: #1f2a37; --line: #dbe3ee; --accent: #1f6feb; }
    body { margin: 0; font-family: "Segoe UI", Tahoma, sans-serif; background: linear-gradient(120deg, #eaf2ff, var(--bg)); color: var(--ink); }
    .wrap { max-width: 900px; margin: 40px auto; padding: 0 20px; }
    .card { background: var(--card); border-radius: 14px; box-shadow: 0 14px 30px rgba(31,42,55,.08); overflow: hidden; }
    header { padding: 22px 24px; border-bottom: 1px solid var(--line); }
    h1 { margin: 0 0 6px; font-size: 24px; }
    .range { margin: 0; color: #4b5563; }
    table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 14px 18px; border-bottom: 1px solid var(--line); }
    th { background: #f8fbff; color: var(--accent); letter-spacing: .02em; }
    tr:hover td { background: #f8fbff; }
  </style>
</head>
<body>
  <main class="wrap">
    <section class="card">
      <header>
        <h1>SLA Report</h1>
        <p class="range">{{.StartTime}} to {{.EndTime}}</p>
      </header>
      <table>
        <thead><tr><th>Metric</th><th>Value</th></tr></thead>
        <tbody>
          {{range .Rows}}<tr><td>{{.Name}}</td><td>{{.Value}}</td></tr>{{end}}
        </tbody>
      </table>
    </section>
  </main>
</body>
</html>`))
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (PDFRenderer) Render(data ReportData) ([]byte, error) {
	if len(data.Rows) == 0 {
		return nil, fmt.Errorf("cannot render PDF for empty report")
	}
	if len(data.Rows) > 1000 {
		return nil, fmt.Errorf("cannot render PDF: too many rows (%d > 1000)", len(data.Rows))
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "SLA Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 7, fmt.Sprintf("%s to %s", data.StartTime, data.EndTime), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	headerH := 8.0
	cellH := 7.0
	metricW := 80.0
	valueW := 106.0

	pdf.SetFillColor(234, 242, 255)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(metricW, headerH, "Metric", "1", 0, "L", true, 0, "")
	pdf.CellFormat(valueW, headerH, "Value", "1", 1, "L", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	for _, row := range data.Rows {
		if pdf.GetY()+cellH > 285 {
			pdf.AddPage()
		}
		pdf.CellFormat(metricW, cellH, row.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(valueW, cellH, row.Value, "1", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func BuildReportData(startTime, endTime time.Time, totalRequests, successRequests, failedRequests int, latencies []time.Duration, slaMetrics []string, latencyPercentiles []int) ReportData {
	rows := []MetricRow{
		{Name: "Total Requests", Value: fmt.Sprintf("%d", totalRequests)},
		{Name: "Successful", Value: fmt.Sprintf("%d", successRequests)},
		{Name: "Failed", Value: fmt.Sprintf("%d", failedRequests)},
	}

	if contains(slaMetrics, "uptime") && totalRequests > 0 {
		uptime := float64(successRequests) / float64(totalRequests) * 100
		rows = append(rows, MetricRow{Name: "Uptime", Value: fmt.Sprintf("%.2f%% (%d/%d successful requests)", uptime, successRequests, totalRequests)})
	}

	if contains(slaMetrics, "error_rate") && totalRequests > 0 {
		errorRate := float64(failedRequests) / float64(totalRequests) * 100
		rows = append(rows, MetricRow{Name: "Error Rate", Value: fmt.Sprintf("%.2f%% (%d/%d failed requests)", errorRate, failedRequests, totalRequests)})
	}

	if contains(slaMetrics, "latency") && len(latencies) > 0 {
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		for _, perc := range latencyPercentiles {
			index := (perc * len(sorted)) / 100
			if index >= len(sorted) {
				index = len(sorted) - 1
			}
			rows = append(rows, MetricRow{Name: fmt.Sprintf("p%d", perc), Value: sorted[index].String()})
		}
	}

	return ReportData{
		StartTime: startTime.Format("2006-01-02 15:04:05"),
		EndTime:   endTime.Format("2006-01-02 15:04:05"),
		Rows:      rows,
	}
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", `\\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
