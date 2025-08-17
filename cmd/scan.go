package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"smanalyzer/pkg/anomaly"
	"smanalyzer/pkg/config"
	"smanalyzer/pkg/istio"
	"smanalyzer/pkg/k8s"
	"smanalyzer/pkg/ml"
	"smanalyzer/pkg/output"
	"smanalyzer/pkg/timeseries"

	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the Service Sesh for anomalies",
	Long: `Scans the Kubernetes Service Mesh (Istio) environment to collect metrics 
and detect anomalies in network traffic, circuit breaking, retries, and timeouts.`,
	Run: runScan,
}

var (
	namespace    string
	duration     time.Duration
	learningMode bool
)

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace to scan (default: all namespaces)")
	scanCmd.Flags().DurationVarP(&duration, "duration", "d", 5*time.Minute, "Duration to scan for (e.g., 5m, 1h)")
	scanCmd.Flags().BoolVarP(&learningMode, "learn", "l", false, "Learning mode - establish baseline behavior patterns")
}

func runScan(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	fmt.Printf("Starting Service Mesh scan...\n")
	if namespace != "" {
		fmt.Printf("Namespace: %s\n", namespace)
	} else {
		fmt.Printf("Scanning all namespaces\n")
	}
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Learning mode: %v\n", learningMode)

	if err := performScan(ctx); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}
}

func performScan(ctx context.Context) error {
	fmt.Println("Connecting to Kubernetes cluster...")

	k8sClient, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	if err := k8sClient.CheckConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	fmt.Println("✓ Connected to Kubernetes cluster")
	fmt.Println("Discovering Services in Mesh...")

	discovery := istio.NewServiceDiscovery(k8sClient.Clientset)
	services, err := discovery.DiscoverServices(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to discover services: %w", err)
	}

	fmt.Printf("✓ Found %d services with Istio sidecars\n", len(services))

	storage := timeseries.NewStorage()
	config := config.DefaultConfig()
	mlConfig := config.ToMLConfig()
	detectionConfig := config.ToAnomalyDetectionConfig()

	clusteringEngine := ml.NewClusteringEngine(mlConfig)
	detector := anomaly.NewDetector(detectionConfig, clusteringEngine)
	formatter := output.NewFormatter(config.Output.Format)

	fmt.Println("Collecting service mesh metrics...")

	var allAnomalies []anomaly.Anomaly

	for _, serviceName := range services {
		metrics, err := discovery.CollectMetrics(ctx, namespace, serviceName)
		if err != nil {
			fmt.Printf("Warning: failed to collect metrics for %s: %v\n", serviceName, err)
			continue
		}

		storage.Store(serviceName, "request_count", float64(metrics.RequestCount), metrics.Labels)
		storage.Store(serviceName, "error_rate", metrics.ErrorRate, metrics.Labels)
		storage.Store(serviceName, "response_time", float64(metrics.ResponseTime.Milliseconds()), metrics.Labels)

		recentPoints := storage.GetLatestN(serviceName, "request_count", 50)

		if learningMode {
			if len(recentPoints) >= detectionConfig.WindowSize {
				if err := detector.LearnBaseline(serviceName, recentPoints); err != nil {
					fmt.Printf("Warning: failed to learn baseline for %s: %v\n", serviceName, err)
				} else {
					fmt.Printf("✓ Learned baseline for %s\n", serviceName)
				}
			}
		} else {
			anomalies, err := detector.DetectAnomalies(serviceName, recentPoints)
			if err != nil {
				fmt.Printf("Warning: failed to detect anomalies for %s: %v\n", serviceName, err)
				continue
			}
			allAnomalies = append(allAnomalies, anomalies...)
		}
	}

	if !learningMode {
		fmt.Printf("\n%s", formatter.FormatAnomalies(allAnomalies))
	}

	return nil
}
