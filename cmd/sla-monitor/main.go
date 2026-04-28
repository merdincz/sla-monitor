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

type appMonitor interface {
	Start()
	Stop()
	ReportData() monitor.ReportData
}

type monitorFactory func(*config.Config, io.Writer) appMonitor
type signalWaiter func() os.Signal

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr, defaultMonitorFactory, waitForSignal).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(out, errOut io.Writer, newMonitor monitorFactory, wait signalWaiter) *cobra.Command {
	var (
		cfgFile            string
		target             string
		concurrency        int
		interval           string
		slaMetrics         []string
		latencyPercentiles []int
		output             string
	)

	rootCmd := &cobra.Command{
		Use:     "sla-monitor",
		Short:   "SLA Monitor is a CLI tool to monitor service level agreements",
		Example: `sla-monitor --target http://localhost:8080 --concurrency 10 --interval 5s --sla_metrics uptime,latency,error_rate --latency_percentiles 50,95,99 --output text`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if output != "text" && output != "html" {
				return fmt.Errorf("invalid output %q: supported values are text and html", output)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bindFlags(cmd); err != nil {
				return err
			}

			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}

			mon := newMonitor(cfg, errOut)

			fmt.Fprintf(errOut, "Target: %s\n", cfg.Target)
			fmt.Fprintf(errOut, "Concurrency: %d\n", cfg.Concurrency)
			fmt.Fprintf(errOut, "Interval: %v\n", cfg.Interval)

			go mon.Start()
			wait()

			fmt.Fprintln(errOut, "\nShutting down monitor...")
			mon.Stop()
			return renderReport(out, output, mon.ReportData())
		},
	}

	rootCmd.SetOut(out)
	rootCmd.SetErr(errOut)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "configs/config.yaml", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&target, "target", "", "Target endpoint")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 0, "Number of concurrent requests")
	rootCmd.PersistentFlags().StringVar(&interval, "interval", "", "Interval duration between requests (e.g., '5s', '1m')")
	rootCmd.PersistentFlags().StringSliceVar(&slaMetrics, "sla_metrics", nil, "SLA metrics to monitor (e.g., uptime,latency,error_rate)")
	rootCmd.PersistentFlags().IntSliceVar(&latencyPercentiles, "latency_percentiles", nil, "Latency percentiles (e.g., 50,95,99)")
	rootCmd.PersistentFlags().StringVar(&output, "output", "text", "Output format: text or html")

	return rootCmd
}

func bindFlags(cmd *cobra.Command) error {
	flags := []string{"target", "concurrency", "interval", "sla_metrics", "latency_percentiles"}
	for _, name := range flags {
		if err := viper.BindPFlag(name, cmd.PersistentFlags().Lookup(name)); err != nil {
			return err
		}
	}
	return nil
}

func defaultMonitorFactory(cfg *config.Config, status io.Writer) appMonitor {
	return monitor.NewMonitorWithStatusWriter(cfg, status)
}

func waitForSignal() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	return <-sigChan
}

func renderReport(w io.Writer, output string, data monitor.ReportData) error {
	switch output {
	case "text":
		return report.RenderText(w, data)
	case "html":
		return report.RenderHTML(w, data)
	default:
		return fmt.Errorf("invalid output %q: supported values are text and html", output)
	}
}
