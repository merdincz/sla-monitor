package report

import (
	"bytes"
	"embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

type LatencyMetric struct {
	Percentile int
	Duration   time.Duration
}

type ReportData struct {
	StartTime          time.Time
	EndTime            time.Time
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	Uptime             *float64
	ErrorRate          *float64
	Latency            []LatencyMetric
}

type Reporter interface {
	Report(ReportData) error
}

type stdoutReporter struct {
	writer io.Writer
}

type fileReporter struct {
	extension string
	outputDir string
	baseName  string
	render    func(ReportData) ([]byte, error)
}

type pdfReporter struct {
	outputDir string
	baseName  string
}

//go:embed templates/report.html.tmpl
var templateFS embed.FS

func NewReporters(outputs []string, outputDir, outputName string, stdout io.Writer) []Reporter {
	reporters := []Reporter{&stdoutReporter{writer: stdout}}

	for _, output := range outputs {
		switch strings.ToLower(strings.TrimSpace(output)) {
		case "", "stdout":
			continue
		case "csv":
			reporters = append(reporters, &fileReporter{
				extension: "csv",
				outputDir: outputDir,
				baseName:  outputName,
				render:    renderCSV,
			})
		case "html":
			reporters = append(reporters, &fileReporter{
				extension: "html",
				outputDir: outputDir,
				baseName:  outputName,
				render:    renderHTML,
			})
		case "md":
			reporters = append(reporters, &fileReporter{
				extension: "md",
				outputDir: outputDir,
				baseName:  outputName,
				render:    renderMarkdown,
			})
		case "pdf":
			reporters = append(reporters, &pdfReporter{outputDir: outputDir, baseName: outputName})
		}
	}

	return reporters
}

func (r *stdoutReporter) Report(data ReportData) error {
	_, err := r.writer.Write(mustRenderMarkdown(data))
	return err
}

func (r *fileReporter) Report(data ReportData) error {
	content, err := r.render(data)
	if err != nil {
		return err
	}
	return writeFile(r.outputDir, r.baseName, r.extension, content)
}

func (r *pdfReporter) Report(data ReportData) error {
	if err := os.MkdirAll(r.outputDir, 0o755); err != nil {
		return err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "SLA Report")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(0, 8, fmt.Sprintf("Window: %s to %s", data.StartTime.Format("2006-01-02 15:04:05"), data.EndTime.Format("2006-01-02 15:04:05")))
	pdf.Ln(10)

	rows := [][]string{{"Total Requests", fmt.Sprintf("%d", data.TotalRequests)}, {"Successful", fmt.Sprintf("%d", data.SuccessfulRequests)}, {"Failed", fmt.Sprintf("%d", data.FailedRequests)}}
	if data.Uptime != nil {
		rows = append(rows, []string{"Uptime", fmt.Sprintf("%.2f%%", *data.Uptime)})
	}
	if data.ErrorRate != nil {
		rows = append(rows, []string{"Error Rate", fmt.Sprintf("%.2f%%", *data.ErrorRate)})
	}

	for _, row := range rows {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(50, 8, row[0], "1", 0, "", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.CellFormat(0, 8, row[1], "1", 1, "", false, 0, "")
	}

	if len(data.Latency) > 0 {
		pdf.Ln(6)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(0, 8, "Latency Percentiles")
		pdf.Ln(8)
		for _, metric := range data.Latency {
			pdf.SetFont("Arial", "", 11)
			pdf.CellFormat(40, 8, fmt.Sprintf("p%d", metric.Percentile), "1", 0, "", false, 0, "")
			pdf.CellFormat(0, 8, metric.Duration.String(), "1", 1, "", false, 0, "")
		}
	}

	return pdf.OutputFileAndClose(filepath.Join(r.outputDir, fmt.Sprintf("%s.%s", r.baseName, "pdf")))
}

func renderCSV(data ReportData) ([]byte, error) {
	buffer := &bytes.Buffer{}
	w := csv.NewWriter(buffer)
	rows := [][]string{{"metric", "value"}, {"total_requests", fmt.Sprintf("%d", data.TotalRequests)}, {"successful_requests", fmt.Sprintf("%d", data.SuccessfulRequests)}, {"failed_requests", fmt.Sprintf("%d", data.FailedRequests)}, {"window_start", data.StartTime.Format(time.RFC3339)}, {"window_end", data.EndTime.Format(time.RFC3339)}}
	if data.Uptime != nil {
		rows = append(rows, []string{"uptime_pct", fmt.Sprintf("%.2f", *data.Uptime)})
	}
	if data.ErrorRate != nil {
		rows = append(rows, []string{"error_rate_pct", fmt.Sprintf("%.2f", *data.ErrorRate)})
	}
	for _, metric := range data.Latency {
		rows = append(rows, []string{fmt.Sprintf("latency_p%d", metric.Percentile), metric.Duration.String()})
	}

	if err := w.WriteAll(rows); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func renderMarkdown(data ReportData) ([]byte, error) {
	return mustRenderMarkdown(data), nil
}

func mustRenderMarkdown(data ReportData) []byte {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("SLA Report (%s to %s)\n", data.StartTime.Format("2006-01-02 15:04:05"), data.EndTime.Format("2006-01-02 15:04:05")))
	b.WriteString("--------------------------------------------------------\n")
	b.WriteString(fmt.Sprintf("Total Requests: %d\n", data.TotalRequests))
	b.WriteString(fmt.Sprintf("Successful: %d\n", data.SuccessfulRequests))
	b.WriteString(fmt.Sprintf("Failed: %d\n", data.FailedRequests))
	if data.Uptime != nil {
		b.WriteString(fmt.Sprintf("Uptime: %.2f%% (%d/%d successful requests)\n", *data.Uptime, data.SuccessfulRequests, data.TotalRequests))
	}
	if data.ErrorRate != nil {
		b.WriteString(fmt.Sprintf("Error Rate: %.2f%% (%d/%d failed requests)\n", *data.ErrorRate, data.FailedRequests, data.TotalRequests))
	}
	if len(data.Latency) > 0 {
		b.WriteString("Latency Metrics\n")
		for _, metric := range data.Latency {
			b.WriteString(fmt.Sprintf("\tp%d: %s\n", metric.Percentile, metric.Duration))
		}
	}
	return []byte(b.String())
}

func renderHTML(data ReportData) ([]byte, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/report.html.tmpl")
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	if err := tmpl.Execute(buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeFile(outputDir, outputName, extension string, content []byte) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, fmt.Sprintf("%s.%s", outputName, extension)), content, 0o644)
}
