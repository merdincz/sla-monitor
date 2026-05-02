package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"sla-monitor/internal/config"
	"sla-monitor/internal/monitor"
	"sla-monitor/internal/report"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile            string
	target             string
	concurrency        int
	interval           string
	slaMetrics         []string
	latencyPercentiles []int
	outputFormat       string
	outputFile         string
)

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "sla-monitor",
		Short:   "SLA Monitor is a CLI tool to monitor service level agreements",
		Example: `sla-monitor --target http://localhost:8080 --concurrency 10 --interval 5s --sla_metrics uptime,latency,error_rate --latency_percentiles 50,95,99 --output-format html --output-file report.html`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, stdout, stderr)
		},
		SilenceUsage: true,
	}

	// Flags for configuration.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "configs/config.yaml", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&target, "target", "", "Target endpoint")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 0, "Number of concurrent requests")
	rootCmd.PersistentFlags().StringVar(&interval, "interval", "", "Interval duration between requests (e.g., '5s', '1m')")
	rootCmd.PersistentFlags().StringSliceVar(&slaMetrics, "sla_metrics", nil, "SLA metrics to monitor (e.g., uptime,latency,error_rate)")
	rootCmd.PersistentFlags().IntSliceVar(&latencyPercentiles, "latency_percentiles", nil, "Latency percentiles (e.g., 50,95,99)")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", string(report.FormatText), "Output format (text, csv, md, html, pdf)")
	rootCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "Write report output to a file")

	return rootCmd
}

func run(cmd *cobra.Command, stdout, stderr io.Writer) error {
	if err := bindConfigFlags(cmd); err != nil {
		return err
	}

	format, err := report.ParseFormat(outputFormat)
	if err != nil {
		return err
	}

	reportWriter, closeReportWriter, err := openReportWriter(format, outputFile, stdout)
	if err != nil {
		return err
	}
	defer closeReportWriter()

	// Load configuration (config file takes precedence if it exists).
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	statusWriter := statusOutputWriter(format, outputFile, stdout, stderr)

	// Create and start the monitor.
	mon := monitor.NewMonitor(cfg)
	mon.SetLogWriter(statusWriter)

	fmt.Fprintf(statusWriter, "Target: %s\n", cfg.Target)
	fmt.Fprintf(statusWriter, "Concurrency: %d\n", cfg.Concurrency)
	fmt.Fprintf(statusWriter, "Interval: %v\n", cfg.Interval)

	monitorDone := make(chan struct{})
	go func() {
		mon.Start()
		close(monitorDone)
	}()

	// Listen for OS termination signals to gracefully shut down.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	<-sigChan

	fmt.Fprintln(statusWriter, "\nShutting down monitor...")
	mon.Stop()
	<-monitorDone

	return report.Render(reportWriter, format, mon.Snapshot())
}

func bindConfigFlags(cmd *cobra.Command) error {
	flagBindings := map[string]string{
		"target":              "target",
		"concurrency":         "concurrency",
		"interval":            "interval",
		"sla_metrics":         "sla_metrics",
		"latency_percentiles": "latency_percentiles",
	}

	for key, flagName := range flagBindings {
		if err := viper.BindPFlag(key, cmd.PersistentFlags().Lookup(flagName)); err != nil {
			return err
		}
	}

	return nil
}

func openReportWriter(format report.Format, filePath string, stdout io.Writer) (io.Writer, func(), error) {
	if format == report.FormatPDF && filePath == "" {
		return nil, func() {}, fmt.Errorf("--output-file is required when --output-format=pdf")
	}

	if filePath == "" {
		return stdout, func() {}, nil
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create output file: %w", err)
	}

	return file, func() { _ = file.Close() }, nil
}

func statusOutputWriter(format report.Format, filePath string, stdout, stderr io.Writer) io.Writer {
	if format == report.FormatText && filePath == "" {
		return stdout
	}
	return stderr
}
