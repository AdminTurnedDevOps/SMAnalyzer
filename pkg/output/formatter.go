package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"smanalyzer/pkg/anomaly"
	"smanalyzer/pkg/istio"
)

type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	Text  Format = "text"
)

type Formatter struct {
	format Format
}

func NewFormatter(format string) *Formatter {
	return &Formatter{format: Format(format)}
}

func (f *Formatter) FormatAnomalies(anomalies []anomaly.Anomaly) string {
	switch f.format {
	case JSON:
		return f.formatJSON(anomalies)
	case Table:
		return f.formatTable(anomalies)
	default:
		return f.formatText(anomalies)
	}
}

func (f *Formatter) formatText(anomalies []anomaly.Anomaly) string {
	if len(anomalies) == 0 {
		return "No anomalies detected.\n"
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d anomalies:\n\n", len(anomalies)))

	for i, anom := range anomalies {
		severity := f.getSeverityText(anom.Severity)
		output.WriteString(fmt.Sprintf("%d. %s [%s]\n", i+1, anom.Description, severity))
		output.WriteString(fmt.Sprintf("   Service: %s.%s\n", anom.ServiceName, anom.Namespace))
		output.WriteString(fmt.Sprintf("   Type: %s\n", anom.Type))
		output.WriteString(fmt.Sprintf("   Time: %s\n", anom.Timestamp.Format(time.RFC3339)))
		
		if len(anom.Metrics) > 0 {
			output.WriteString("   Metrics:\n")
			for key, value := range anom.Metrics {
				output.WriteString(fmt.Sprintf("     %s: %.2f\n", key, value))
			}
		}
		output.WriteString("\n")
	}

	return output.String()
}

func (f *Formatter) formatTable(anomalies []anomaly.Anomaly) string {
	if len(anomalies) == 0 {
		return "No anomalies detected.\n"
	}

	var output strings.Builder
	
	output.WriteString("SERVICE          NAMESPACE    TYPE              SEVERITY  DESCRIPTION\n")
	output.WriteString("-------          ---------    ----              --------  -----------\n")

	for _, anom := range anomalies {
		service := f.truncate(anom.ServiceName, 15)
		namespace := f.truncate(anom.Namespace, 11)
		anomType := f.truncate(string(anom.Type), 16)
		severity := f.getSeverityText(anom.Severity)
		description := f.truncate(anom.Description, 40)

		output.WriteString(fmt.Sprintf("%-15s  %-11s  %-16s  %-8s  %s\n", 
			service, namespace, anomType, severity, description))
	}

	return output.String()
}

func (f *Formatter) formatJSON(anomalies []anomaly.Anomaly) string {
	return fmt.Sprintf("%+v\n", anomalies)
}

func (f *Formatter) getSeverityText(severity float64) string {
	if severity >= 3.0 {
		return "CRITICAL"
	} else if severity >= 2.0 {
		return "HIGH"
	} else if severity >= 1.5 {
		return "MEDIUM"
	}
	return "LOW"
}

func (f *Formatter) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (f *Formatter) DisplayMetrics(metrics []*istio.ServiceMeshMetrics) error {
	switch f.format {
	case JSON:
		return f.displayMetricsJSON(metrics)
	case Table:
		return f.displayMetricsTable(metrics)
	default:
		return f.displayMetricsText(metrics)
	}
}

func (f *Formatter) displayMetricsText(metrics []*istio.ServiceMeshMetrics) error {
	if len(metrics) == 0 {
		fmt.Printf("[%s] No services found\n", time.Now().Format("15:04:05"))
		return nil
	}

	fmt.Printf("[%s] Service Mesh Metrics:\n\n", time.Now().Format("15:04:05"))
	
	for _, m := range metrics {
		fmt.Printf("Service: %s.%s\n", m.ServiceName, m.Namespace)
		fmt.Printf("  Request Count: %d\n", m.RequestCount)
		fmt.Printf("  Error Rate: %.2f%%\n", m.ErrorRate*100)
		fmt.Printf("  Response Time: %v\n", m.ResponseTime)
		fmt.Printf("  Circuit Breakers: %d\n", m.CircuitBreakers)
		fmt.Printf("  Retry Count: %d\n", m.RetryCount)
		fmt.Printf("  Timeout Count: %d\n", m.TimeoutCount)
		fmt.Println()
	}
	
	return nil
}

func (f *Formatter) displayMetricsTable(metrics []*istio.ServiceMeshMetrics) error {
	if len(metrics) == 0 {
		fmt.Printf("[%s] No services found\n", time.Now().Format("15:04:05"))
		return nil
	}

	fmt.Printf("[%s] Service Mesh Metrics:\n\n", time.Now().Format("15:04:05"))
	fmt.Printf("%-20s %-10s %-12s %-8s %-12s %-8s %-8s %-8s\n", 
		"SERVICE", "NAMESPACE", "REQ_COUNT", "ERR_RATE", "RESP_TIME", "CIRCUIT", "RETRIES", "TIMEOUTS")
	fmt.Printf("%-20s %-10s %-12s %-8s %-12s %-8s %-8s %-8s\n", 
		"-------", "---------", "---------", "--------", "---------", "-------", "-------", "--------")
	
	for _, m := range metrics {
		service := f.truncate(m.ServiceName, 19)
		namespace := f.truncate(m.Namespace, 9)
		
		fmt.Printf("%-20s %-10s %-12d %-8.2f %-12v %-8d %-8d %-8d\n",
			service, namespace, m.RequestCount, m.ErrorRate*100, 
			m.ResponseTime, m.CircuitBreakers, m.RetryCount, m.TimeoutCount)
	}
	fmt.Println()
	
	return nil
}

func (f *Formatter) displayMetricsJSON(metrics []*istio.ServiceMeshMetrics) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}
	
	fmt.Printf("[%s] Service Mesh Metrics (JSON):\n", time.Now().Format("15:04:05"))
	fmt.Println(string(data))
	fmt.Println()
	
	return nil
}