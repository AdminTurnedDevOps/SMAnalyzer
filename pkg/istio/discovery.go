package istio

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type ServiceDiscovery struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
	httpClient *http.Client
}

type ServiceMeshMetrics struct {
	ServiceName string `json:"service_name"`
	Namespace   string `json:"namespace"`

	// Four Golden Signals (Istio standard)
	Latency    LatencyMetrics    `json:"latency"`    // Response time distribution
	Traffic    TrafficMetrics    `json:"traffic"`    // Request volume
	Errors     ErrorMetrics      `json:"errors"`     // Error rates by type
	Saturation SaturationMetrics `json:"saturation"` // Resource utilization

	// Service mesh specific
	CircuitBreakers int   `json:"circuit_breakers"`
	RetryCount      int64 `json:"retry_count"`
	TimeoutCount    int64 `json:"timeout_count"`

	// Observability data
	Traces     []TraceSpan      `json:"traces"`
	AccessLogs []AccessLogEntry `json:"access_logs"`

	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
}

type LatencyMetrics struct {
	P50  time.Duration `json:"p50"`
	P90  time.Duration `json:"p90"`
	P95  time.Duration `json:"p95"`
	P99  time.Duration `json:"p99"`
	Mean time.Duration `json:"mean"`
}

type TrafficMetrics struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	TotalRequests     int64   `json:"total_requests"`
	InboundBytes      int64   `json:"inbound_bytes"`
	OutboundBytes     int64   `json:"outbound_bytes"`
}

type ErrorMetrics struct {
	ErrorRate    float64 `json:"error_rate"`
	Errors4xx    int64   `json:"errors_4xx"`
	Errors5xx    int64   `json:"errors_5xx"`
	ConnFailures int64   `json:"connection_failures"`
}

type SaturationMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	Connections int64   `json:"active_connections"`
	PendingReqs int64   `json:"pending_requests"`
}

type TraceSpan struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id"`
	Operation    string            `json:"operation"`
	StartTime    time.Time         `json:"start_time"`
	Duration     time.Duration     `json:"duration"`
	Tags         map[string]string `json:"tags"`
	Status       string            `json:"status"`
}

type AccessLogEntry struct {
	Timestamp     time.Time     `json:"timestamp"`
	Method        string        `json:"method"`
	Path          string        `json:"path"`
	StatusCode    int           `json:"status_code"`
	ResponseTime  time.Duration `json:"response_time"`
	RequestSize   int64         `json:"request_size"`
	ResponseSize  int64         `json:"response_size"`
	UserAgent     string        `json:"user_agent"`
	SourceIP      string        `json:"source_ip"`
	DestinationIP string        `json:"destination_ip"`
}

func NewServiceDiscovery(clientset *kubernetes.Clientset, restConfig *rest.Config) *ServiceDiscovery {
	return &ServiceDiscovery{
		clientset:  clientset,
		restConfig: restConfig,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (sd *ServiceDiscovery) DiscoverServices(ctx context.Context, namespace string) ([]string, error) {
	// First check Istio control plane health
	if err := sd.checkControlPlaneHealth(ctx); err != nil {
		fmt.Printf("Warning: Istio control plane issues detected: %v\n", err)
	}

	fmt.Printf("Debug: DiscoverServices called with namespace='%s'\n", namespace)

	// Get pods with Istio sidecars instead of services
	listOptions := metav1.ListOptions{}
	searchNamespace := namespace
	if namespace == "" {
		searchNamespace = metav1.NamespaceAll
	}

	fmt.Printf("Debug: Searching in namespace='%s'\n", searchNamespace)

	pods, err := sd.clientset.CoreV1().Pods(searchNamespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	fmt.Printf("Debug: Found %d total pods in namespace '%s'\n", len(pods.Items), searchNamespace)

	serviceSet := make(map[string]bool)
	for _, pod := range pods.Items {
		if hasIstioSidecar(pod.Labels, pod.Annotations) {
			// Extract service name from app label or pod name
			if serviceName := getServiceName(pod.Labels); serviceName != "" {
				// Include namespace in service identifier for cross-namespace scanning
				serviceKey := fmt.Sprintf("%s.%s", serviceName, pod.Namespace)
				serviceSet[serviceKey] = true
				fmt.Printf("Debug: Found Istio service: %s\n", serviceKey)
			}
		}
	}

	var serviceNames []string
	for serviceKey := range serviceSet {
		serviceNames = append(serviceNames, serviceKey)
	}

	return serviceNames, nil
}

func (sd *ServiceDiscovery) CollectMetrics(ctx context.Context, namespace, serviceName string) (*ServiceMeshMetrics, error) {
	metrics := &ServiceMeshMetrics{
		ServiceName: serviceName,
		Namespace:   namespace,
		Timestamp:   time.Now(),
		Labels:      make(map[string]string),
	}

	// Find pods for this service
	pods, err := sd.getServicePods(ctx, namespace, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pods for service %s: %w", serviceName, err)
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("no pods found for service %s", serviceName)
	}

	// Collect metrics from the first available pod (could aggregate across all pods)
	for _, pod := range pods {
		fmt.Printf("  Attempting to collect metrics from pod %s\n", pod)
		if err := sd.collectEnvoyMetrics(ctx, pod, metrics); err != nil {
			fmt.Printf("  Failed to collect metrics from pod %s: %v\n", pod, err)
			continue // Try next pod if this one fails
		}
		fmt.Printf("  ✓ Successfully collected metrics from pod %s\n", pod)
		return metrics, nil
	}

	return nil, fmt.Errorf("failed to collect metrics from any pod for service %s", serviceName)
}

func (sd *ServiceDiscovery) getServicePods(ctx context.Context, namespace, serviceName string) ([]string, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", serviceName),
	}

	pods, err := sd.clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var podNames []string
	for _, pod := range pods.Items {
		if hasIstioSidecar(pod.Labels, pod.Annotations) && pod.Status.Phase == "Running" {
			podNames = append(podNames, pod.Name)
		}
	}
	return podNames, nil
}

func (sd *ServiceDiscovery) collectEnvoyMetrics(ctx context.Context, podName string, metrics *ServiceMeshMetrics) error {
	// Use kubectl exec to access Istio's Prometheus metrics endpoint
	// This endpoint exposes Envoy metrics in Prometheus format on port 15020

	// Execute curl command to get Prometheus metrics from istio-proxy container
	cmd := []string{"curl", "-s", "http://localhost:15020/stats/prometheus"}

	// Create the exec request using the proper API
	req := sd.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(metrics.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "istio-proxy",
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, runtime.NewParameterCodec(scheme.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(sd.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return fmt.Errorf("failed to execute command: %w (stderr: %s)", err, stderr.String())
	}

	if stderr.Len() > 0 {
		return fmt.Errorf("command stderr: %s", stderr.String())
	}

	metricsOutput := stdout.String()
	if len(metricsOutput) == 0 {
		return fmt.Errorf("no metrics output received from pod %s", podName)
	}

	return sd.parsePrometheusMetrics(metricsOutput, metrics)
}

func (sd *ServiceDiscovery) parsePrometheusMetrics(prometheusText string, metrics *ServiceMeshMetrics) error {
	lines := strings.Split(prometheusText, "\n")

	var requestTotal, errors4xx, errors5xx float64
	var p50, p90, p95, p99 float64
	var inboundBytes, outboundBytes float64
	var connections, pendingReqs float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip comments and empty lines
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		metricName := parts[0]
		valueStr := parts[1]
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		// Parse Istio/Envoy metrics
		if strings.Contains(metricName, "istio_requests_total") {
			if strings.Contains(metricName, "response_code=\"200\"") ||
				strings.Contains(metricName, "response_code=\"2") {
				requestTotal += value
			} else if strings.Contains(metricName, "response_code=\"4") {
				errors4xx += value
			} else if strings.Contains(metricName, "response_code=\"5") {
				errors5xx += value
			}
		}

		// Parse request duration percentiles
		if strings.Contains(metricName, "istio_request_duration_milliseconds") {
			if strings.Contains(metricName, "quantile=\"0.5\"") {
				p50 = value
			} else if strings.Contains(metricName, "quantile=\"0.9\"") {
				p90 = value
			} else if strings.Contains(metricName, "quantile=\"0.95\"") {
				p95 = value
			} else if strings.Contains(metricName, "quantile=\"0.99\"") {
				p99 = value
			}
		}

		// Parse connection metrics
		if strings.Contains(metricName, "envoy_http_downstream_cx_active") {
			connections = value
		}

		// Parse bytes transferred
		if strings.Contains(metricName, "istio_request_bytes") {
			inboundBytes += value
		}
		if strings.Contains(metricName, "istio_response_bytes") {
			outboundBytes += value
		}

		// Parse circuit breaker metrics
		if strings.Contains(metricName, "envoy_cluster_upstream_rq_retry") {
			metrics.RetryCount = int64(value)
		}
		if strings.Contains(metricName, "envoy_cluster_upstream_rq_timeout") {
			metrics.TimeoutCount = int64(value)
		}
		if strings.Contains(metricName, "envoy_cluster_circuit_breakers") && strings.Contains(metricName, "cx_open") {
			metrics.CircuitBreakers = int(value)
		}
	}

	// Populate structured metrics
	totalRequests := requestTotal + errors4xx + errors5xx
	metrics.Traffic = TrafficMetrics{
		TotalRequests:     int64(totalRequests),
		RequestsPerSecond: totalRequests / 60, // Approximate RPS over last minute
		InboundBytes:      int64(inboundBytes),
		OutboundBytes:     int64(outboundBytes),
	}

	metrics.Latency = LatencyMetrics{
		P50:  time.Duration(p50) * time.Millisecond,
		P90:  time.Duration(p90) * time.Millisecond,
		P95:  time.Duration(p95) * time.Millisecond,
		P99:  time.Duration(p99) * time.Millisecond,
		Mean: time.Duration((p50+p90+p95+p99)/4) * time.Millisecond, // Approximate mean
	}

	errorRate := float64(0)
	if totalRequests > 0 {
		errorRate = ((errors4xx + errors5xx) / totalRequests) * 100
	}

	metrics.Errors = ErrorMetrics{
		ErrorRate: errorRate,
		Errors4xx: int64(errors4xx),
		Errors5xx: int64(errors5xx),
	}

	metrics.Saturation = SaturationMetrics{
		Connections: int64(connections),
		PendingReqs: int64(pendingReqs),
		// CPU/Memory would need additional pod metrics
		CPUUsage:    0,
		MemoryUsage: 0,
	}

	// Initialize observability arrays (real implementation would parse traces/logs)
	metrics.Traces = []TraceSpan{}
	metrics.AccessLogs = []AccessLogEntry{}

	// Debug output showing real metrics collected
	fmt.Printf("    📊 Metrics collected: Requests=%d, RPS=%.1f, Errors=%.2f%%, P99=%v\n",
		metrics.Traffic.TotalRequests,
		metrics.Traffic.RequestsPerSecond,
		metrics.Errors.ErrorRate,
		metrics.Latency.P99)

	return nil
}

func (sd *ServiceDiscovery) parseEnvoyStatsText(statsText string, metrics *ServiceMeshMetrics) error {
	lines := strings.Split(statsText, "\n")

	var totalRequests, errors4xx, errors5xx float64
	var p50, p90, p95, p99, mean float64
	var inboundBytes, outboundBytes float64
	var connections, pendingReqs float64

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		statName := parts[0]
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}

		// Traffic metrics
		if strings.Contains(statName, "http.inbound.rq_completed") {
			totalRequests = value
		}
		if strings.Contains(statName, "http.inbound.downstream_rq_total") {
			inboundBytes = value
		}
		if strings.Contains(statName, "http.outbound.upstream_rq_total") {
			outboundBytes = value
		}

		// Error metrics
		if strings.Contains(statName, "http.inbound.rq_4xx") {
			errors4xx += value
		}
		if strings.Contains(statName, "http.inbound.rq_5xx") {
			errors5xx += value
		}

		// Latency metrics (histogram percentiles)
		if strings.Contains(statName, "http.inbound.downstream_rq_time") {
			if strings.Contains(statName, "P50") {
				p50 = value
			} else if strings.Contains(statName, "P90") {
				p90 = value
			} else if strings.Contains(statName, "P95") {
				p95 = value
			} else if strings.Contains(statName, "P99") {
				p99 = value
			} else if strings.Contains(statName, "mean") {
				mean = value
			}
		}

		// Saturation metrics
		if strings.Contains(statName, "server.live") {
			connections = value
		}
		if strings.Contains(statName, "http.inbound.downstream_rq_pending_total") {
			pendingReqs = value
		}

		// Service mesh specific
		if strings.Contains(statName, "upstream_rq_timeout") {
			metrics.TimeoutCount = int64(value)
		}
		if strings.Contains(statName, "upstream_rq_retry") {
			metrics.RetryCount = int64(value)
		}
		if strings.Contains(statName, "circuit_breakers") && strings.Contains(statName, "open") {
			metrics.CircuitBreakers = int(value)
		}
	}

	// Populate structured metrics
	metrics.Traffic = TrafficMetrics{
		TotalRequests:     int64(totalRequests),
		RequestsPerSecond: totalRequests / 60, // Approximate RPS
		InboundBytes:      int64(inboundBytes),
		OutboundBytes:     int64(outboundBytes),
	}

	metrics.Latency = LatencyMetrics{
		P50:  time.Duration(p50) * time.Millisecond,
		P90:  time.Duration(p90) * time.Millisecond,
		P95:  time.Duration(p95) * time.Millisecond,
		P99:  time.Duration(p99) * time.Millisecond,
		Mean: time.Duration(mean) * time.Millisecond,
	}

	errorRate := float64(0)
	if totalRequests > 0 {
		errorRate = ((errors4xx + errors5xx) / totalRequests) * 100
	}

	metrics.Errors = ErrorMetrics{
		ErrorRate: errorRate,
		Errors4xx: int64(errors4xx),
		Errors5xx: int64(errors5xx),
	}

	metrics.Saturation = SaturationMetrics{
		Connections: int64(connections),
		PendingReqs: int64(pendingReqs),
		// CPU/Memory would need additional pod metrics
		CPUUsage:    0,
		MemoryUsage: 0,
	}

	return nil
}

func (sd *ServiceDiscovery) parseEnvoyStatsJSON(stats map[string]interface{}, metrics *ServiceMeshMetrics) error {
	// Parse JSON format stats (implementation depends on Envoy version)
	// For now, use text parsing fallback
	return fmt.Errorf("JSON stats parsing not implemented, use text format")
}

func hasIstioSidecar(labels, annotations map[string]string) bool {
	// Check pod annotations for sidecar injection
	if annotations != nil {
		if val, exists := annotations["sidecar.istio.io/status"]; exists && val != "" {
			return true
		}
	}

	// Check labels for injection enabled
	if labels != nil {
		if val, exists := labels["istio-injection"]; exists && val == "enabled" {
			return true
		}
		if val, exists := labels["sidecar.istio.io/inject"]; exists && val == "true" {
			return true
		}
	}

	return false
}

func getServiceName(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	// Try app label first (most common)
	if serviceName, exists := labels["app"]; exists {
		return serviceName
	}

	// Try app.kubernetes.io/name
	if serviceName, exists := labels["app.kubernetes.io/name"]; exists {
		return serviceName
	}

	// Try service label
	if serviceName, exists := labels["service"]; exists {
		return serviceName
	}

	return ""
}

// Istio Control Plane Health Monitoring
func (sd *ServiceDiscovery) checkControlPlaneHealth(ctx context.Context) error {
	// Check for Istio system namespace
	istioNamespace := "istio-system"

	// Check Pilot (istiod) health
	pilots, err := sd.clientset.AppsV1().Deployments(istioNamespace).Get(ctx, "istiod", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("pilot (istiod) not found: %w", err)
	}

	if pilots.Status.ReadyReplicas < 1 {
		return fmt.Errorf("pilot (istiod) not ready: %d/%d replicas", pilots.Status.ReadyReplicas, pilots.Status.Replicas)
	}

	// Check Istio ingress gateway
	_, err = sd.clientset.AppsV1().Deployments(istioNamespace).Get(ctx, "istio-ingressgateway", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Warning: Istio ingress gateway not found: %v\n", err)
	}

	fmt.Printf("✓ Istio control plane healthy (Pilot: %d replicas)\n", pilots.Status.ReadyReplicas)
	return nil
}

// Real Envoy Admin API data collection functions
func (sd *ServiceDiscovery) collectEnvoyStats(ctx context.Context, podName string, metrics *ServiceMeshMetrics) error {
	// Use kubectl exec to access Envoy admin from within the pod
	// This is the most reliable way since admin interface is localhost-only

	fmt.Printf("    Executing curl inside pod %s\n", podName)

	// Execute curl command inside the istio-proxy container
	cmd := []string{"curl", "-s", "http://localhost:15000/stats"}

	req := sd.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(metrics.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "istio-proxy", // Execute in the Envoy sidecar container
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(sd.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return fmt.Errorf("failed to execute command: %w (stderr: %s)", err, stderr.String())
	}

	if stderr.Len() > 0 {
		fmt.Printf("    Warning: %s\n", stderr.String())
	}

	statsOutput := stdout.String()
	if len(statsOutput) == 0 {
		return fmt.Errorf("no stats output received from pod %s", podName)
	}

	fmt.Printf("    ✓ Retrieved %d bytes of Envoy stats\n", len(statsOutput))

	return sd.parseEnvoyStatsText(statsOutput, metrics)
}

func (sd *ServiceDiscovery) collectEnvoyAccessLogs(ctx context.Context, envoyAdminURL string, metrics *ServiceMeshMetrics) error {
	// Envoy access logs are typically written to files or stdout, not exposed via admin API
	// In a real implementation, you would:
	// 1. Read from pod's log stream
	// 2. Parse JSON access log format
	// 3. Extract request details

	logsURL := envoyAdminURL + "/logging"
	resp, err := sd.httpClient.Get(logsURL)
	if err != nil {
		// Access logs might not be available via admin API
		return nil
	}
	defer resp.Body.Close()

	// Parse any available log data
	// For now, initialize empty logs array
	metrics.AccessLogs = []AccessLogEntry{}
	return nil
}

func (sd *ServiceDiscovery) collectEnvoyTraces(ctx context.Context, envoyAdminURL string, metrics *ServiceMeshMetrics) error {
	// Distributed traces are typically sent to external systems (Jaeger, Zipkin)
	// Envoy admin API might expose some trace configuration but not the spans themselves
	// In a real implementation, you would:
	// 1. Connect to Jaeger/Zipkin API
	// 2. Query for traces related to this service
	// 3. Parse trace spans

	tracingURL := envoyAdminURL + "/config_dump?resource=dynamic_active_clusters"
	resp, err := sd.httpClient.Get(tracingURL)
	if err != nil {
		// Tracing config might not be available
		return nil
	}
	defer resp.Body.Close()

	// For now, initialize empty traces array
	metrics.Traces = []TraceSpan{}
	return nil
}
