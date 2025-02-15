package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sla-monitor/internal/config"
	"sla-monitor/internal/monitor"

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

			// Load configuration (config file takes precedence if it exists)
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				os.Exit(1)
			}

			// Create and start the monitor.
			mon := monitor.NewMonitor(cfg)

			fmt.Printf("Target: %s\n", cfg.Target)
			fmt.Printf("Concurrency: %d\n", cfg.Concurrency)
			fmt.Printf("Interval: %v\n", cfg.Interval)

			go mon.Start()

			// Listen for OS termination signals to gracefully shut down.
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan

			fmt.Println("\nShutting down monitor...")
			mon.Stop()
			mon.Report()
		},
	}

	// Flags for configuration.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "configs/config.yaml", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&target, "target", "", "Target endpoint")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 0, "Number of concurrent requests")
	rootCmd.PersistentFlags().StringVar(&interval, "interval", "", "Interval duration between requests (e.g., '5s', '1m')")
	rootCmd.PersistentFlags().StringSliceVar(&slaMetrics, "sla_metrics", nil, "SLA metrics to monitor (e.g., uptime,latency,error_rate)")
	rootCmd.PersistentFlags().IntSliceVar(&latencyPercentiles, "latency_percentiles", nil, "Latency percentiles (e.g., 50,95,99)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
