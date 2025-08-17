package config

import (
	"time"
	"smanalyzer/pkg/anomaly"
	"smanalyzer/pkg/ml"
)

type Config struct {
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Detection  DetectionConfig  `yaml:"detection"`
	Clustering ClusteringConfig `yaml:"clustering"`
	Output     OutputConfig     `yaml:"output"`
}

type KubernetesConfig struct {
	Namespace    string        `yaml:"namespace"`
	LabelSelector string       `yaml:"label_selector"`
	Timeout      time.Duration `yaml:"timeout"`
}

type DetectionConfig struct {
	TrafficSpikeThreshold float64       `yaml:"traffic_spike_threshold"`
	ErrorRateThreshold    float64       `yaml:"error_rate_threshold"`
	LatencyThreshold      time.Duration `yaml:"latency_threshold"`
	RetryThreshold        int64         `yaml:"retry_threshold"`
	TimeoutThreshold      int64         `yaml:"timeout_threshold"`
	WindowSize           int           `yaml:"window_size"`
	SensitivityLevel     float64       `yaml:"sensitivity_level"`
}

type ClusteringConfig struct {
	K           int     `yaml:"k"`
	MaxIter     int     `yaml:"max_iter"`
	Tolerance   float64 `yaml:"tolerance"`
	WindowSize  int     `yaml:"window_size"`
}

type OutputConfig struct {
	Format  string `yaml:"format"`
	Verbose bool   `yaml:"verbose"`
}

func DefaultConfig() *Config {
	return &Config{
		Kubernetes: KubernetesConfig{
			Namespace:     "",
			LabelSelector: "app",
			Timeout:       30 * time.Second,
		},
		Detection: DetectionConfig{
			TrafficSpikeThreshold: 2.0,
			ErrorRateThreshold:    0.05,
			LatencyThreshold:      1 * time.Second,
			RetryThreshold:        100,
			TimeoutThreshold:      10,
			WindowSize:           10,
			SensitivityLevel:     2.0,
		},
		Clustering: ClusteringConfig{
			K:          3,
			MaxIter:    100,
			Tolerance:  0.01,
			WindowSize: 10,
		},
		Output: OutputConfig{
			Format:  "text",
			Verbose: false,
		},
	}
}

func (c *Config) ToAnomalyDetectionConfig() anomaly.DetectionConfig {
	return anomaly.DetectionConfig{
		TrafficSpikeThreshold: c.Detection.TrafficSpikeThreshold,
		ErrorRateThreshold:    c.Detection.ErrorRateThreshold,
		LatencyThreshold:      c.Detection.LatencyThreshold,
		RetryThreshold:        c.Detection.RetryThreshold,
		TimeoutThreshold:      c.Detection.TimeoutThreshold,
		WindowSize:           c.Detection.WindowSize,
		SensitivityLevel:     c.Detection.SensitivityLevel,
	}
}

func (c *Config) ToMLConfig() ml.KMeansConfig {
	return ml.KMeansConfig{
		K:         c.Clustering.K,
		MaxIter:   c.Clustering.MaxIter,
		Tolerance: c.Clustering.Tolerance,
	}
}