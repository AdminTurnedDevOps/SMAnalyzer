package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"smanalyzer/pkg/istio"
	"smanalyzer/pkg/k8s"
	"smanalyzer/pkg/output"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Continuously monitor service mesh metrics",
	Long: `Runs continuous monitoring of the service mesh, displaying real-time metrics
for all services including request counts, error rates, response times, and more.`,
	Run: runMonitor,
}

var (
	monitorInterval  time.Duration
	monitorNamespace string
	outputFormat     string
)

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().DurationVarP(&monitorInterval, "interval", "i", 30*time.Second, "Monitoring interval (e.g., 30s, 1m)")
	monitorCmd.Flags().StringVarP(&monitorNamespace, "namespace", "n", "default", "Kubernetes namespace to monitor")
	monitorCmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "Output format (text, table, json)")
}

func runMonitor(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	fmt.Printf("Starting service mesh monitoring...\n")
	fmt.Printf("Namespace: %s\n", monitorNamespace)
	fmt.Printf("Interval: %v\n", monitorInterval)
	fmt.Printf("Output format: %s\n", outputFormat)
	fmt.Println()

	if err := performMonitoring(ctx); err != nil {
		log.Fatalf("Monitoring failed: %v", err)
	}
}

func performMonitoring(ctx context.Context) error {
	// Connect to Kubernetes cluster
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if err := k8sClient.CheckConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	// Initialize service discovery
	serviceDiscovery := istio.NewServiceDiscovery(k8sClient.Clientset, k8sClient.RestConfig)

	// Initialize output formatter
	formatter := output.NewFormatter(outputFormat)

	fmt.Println("Connected to cluster, starting monitoring loop...")
	fmt.Println()

	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Monitoring stopped")
			return nil
		case <-ticker.C:
			if err := collectAndDisplayMetrics(ctx, serviceDiscovery, formatter); err != nil {
				fmt.Printf("Error collecting metrics: %v\n", err)
			}
		}
	}
}

func collectAndDisplayMetrics(ctx context.Context, sd *istio.ServiceDiscovery, formatter *output.Formatter) error {
	// Discover services
	services, err := sd.DiscoverServices(ctx, monitorNamespace)
	if err != nil {
		return fmt.Errorf("failed to discover services: %w", err)
	}

	fmt.Println("Please Note: This is a continuous monitoring loop. Press CTRL + C to leave")
	fmt.Println()

	for _, s := range services {
		if len(s) == 0 {
			fmt.Printf("[%s] No services found in namespace %s\n", time.Now().Format("15:04:05"), monitorNamespace)
			break
		}
	}

	// Collect metrics for each service
	var allMetrics []*istio.ServiceMeshMetrics
	for _, serviceName := range services {
		metrics, err := sd.CollectMetrics(ctx, monitorNamespace, serviceName)
		if err != nil {
			fmt.Printf("Warning: failed to collect metrics for %s: %v\n", serviceName, err)
			continue
		}
		allMetrics = append(allMetrics, metrics)
	}

	// Display metrics
	return formatter.DisplayMetrics(allMetrics)
}
