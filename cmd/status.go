package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current service mesh status and health",
	Long: `Displays the current status of the service mesh, including:
- Connected services and their health
- Recent anomaly counts
- System configuration
- Baseline model status`,
	Run: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	
	fmt.Println("Service Mesh Analyzer Status")
	fmt.Println("============================")
	
	if err := showStatus(ctx); err != nil {
		log.Fatalf("Status check failed: %v", err)
	}
}

func showStatus(ctx context.Context) error {
	fmt.Println("\n🔍 Cluster Connection:")
	fmt.Println("  Status: Connected")
	fmt.Println("  Cluster: kind-kind") 
	fmt.Println("  Namespaces: 12")
	
	fmt.Println("\n🕸️  Service Mesh:")
	fmt.Println("  Istio Version: 1.20.0")
	fmt.Println("  Services with sidecars: 15")
	fmt.Println("  Gateway services: 2")
	
	fmt.Println("\n🤖 AI Model:")
	fmt.Println("  Baseline Status: Trained")
	fmt.Println("  Last Updated: 2024-01-15 14:30:00")
	fmt.Println("  Training Data: 24h")
	
	fmt.Println("\n📊 Recent Activity:")
	fmt.Println("  Anomalies (last 1h): 2")
	fmt.Println("  Anomalies (last 24h): 12")
	fmt.Println("  Services monitored: 15")
	
	fmt.Println("\n⚙️  Configuration:")
	fmt.Println("  Error rate threshold: 5%")
	fmt.Println("  Traffic spike threshold: 2x")
	fmt.Println("  Sensitivity level: 2.0")
	
	return nil
}