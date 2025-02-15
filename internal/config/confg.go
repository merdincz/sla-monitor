package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Target             string        `mapstructure:"target"`
	Concurrency        int           `mapstructure:"concurrency"`
	Interval           time.Duration `mapstructure:"interval"`
	SLAMetrics         []string      `mapstructure:"sla_metrics"`
	LatencyPercentiles []int         `mapstructure:"latency_percentiles"`
}

// Use config file if it exists, otherwise use CLI flags/env variables.
func LoadConfig(cfgFile string) (*Config, error) {
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		} else {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
