package main

import (
	"fmt"
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
	output             string
	outputFile         string
	reportEngine       string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "sla-monitor",
		Short:   "SLA Monitor is a CLI tool to monitor service level agreements",
		Example: `sla-monitor --target http://localhost:8080 --concurrency 10 --interval 5s --sla_metrics uptime,latency,error_rate --latency_percentiles 50,95,99`,
		Run: func(cmd *cobra.Command, args []string) {

			viper.BindPFlag("target", cmd.PersistentFlags().Lookup("target"))
			viper.BindPFlag("concurrency", cmd.PersistentFlags().Lookup("concurrency"))
			viper.BindPFlag("interval", cmd.PersistentFlags().Lookup("interval"))
			viper.BindPFlag("sla_metrics", cmd.PersistentFlags().Lookup("sla_metrics"))
			viper.BindPFlag("latency_percentiles", cmd.PersistentFlags().Lookup("latency_percentiles"))

			if reportEngine != "legacy" && reportEngine != "v2" {
				fmt.Printf("invalid --report-engine value %q; allowed: legacy,v2\n", reportEngine)
				os.Exit(1)
			}

			if err := report.ValidateOutputConfig(output, outputFile); err != nil {
				fmt.Printf("invalid output configuration: %v\n", err)
				os.Exit(1)
			}

			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				os.Exit(1)
			}

			mon := monitor.NewMonitor(cfg)

			fmt.Printf("Target: %s\n", cfg.Target)
			fmt.Printf("Concurrency: %d\n", cfg.Concurrency)
			fmt.Printf("Interval: %v\n", cfg.Interval)

			go mon.Start()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan

			fmt.Println("\nShutting down monitor...")
			mon.Stop()

			if reportEngine == "legacy" {
				mon.Report()
				return
			}

			if err := mon.ReportV2(output, outputFile); err != nil {
				fmt.Printf("Error generating report: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "configs/config.yaml", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&target, "target", "", "Target endpoint URL")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 0, "Number of concurrent requests")
	rootCmd.PersistentFlags().StringVar(&interval, "interval", "", "Interval duration between requests (e.g., '5s', '1m')")
	rootCmd.PersistentFlags().StringSliceVar(&slaMetrics, "sla_metrics", nil, "SLA metrics to monitor (e.g., uptime,latency,error_rate)")
	rootCmd.PersistentFlags().IntSliceVar(&latencyPercentiles, "latency_percentiles", nil, "Latency percentiles (e.g., 50,95,99)")
	rootCmd.PersistentFlags().StringVar(&output, "output", "stdout", "Output format: stdout|csv|md|html|pdf")
	rootCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "Output file path for csv|md|html|pdf")
	rootCmd.PersistentFlags().StringVar(&reportEngine, "report-engine", "legacy", "Report engine: legacy|v2")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
