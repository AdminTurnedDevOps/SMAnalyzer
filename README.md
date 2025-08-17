# SMAnalyzer

Scans your current cluster to check for anomalies within your L7 networking Kubernetes Services.

![](images/showcase.gif)

## Why It Makes Sense

There are three key aspects to a Service Mesh:
1. Encryption of traffic from service to service (east/west traffic)
2. Traffic routing to/from the service
3. Network observability (performance, circuit breaking, retries, timeouts, load balancing)

Although numbers 1 and 2 are drastically important, number 3 is the make or break between an application performing as expected and angry external or internal (your teammates) customers.

All applications should perform as expected, and typically, bad performance stems from an networking issue (unless it's a specific app/code issue)

**PLEASE NOTE**: This is for scanning real-time/current usage, not a forecasting tool that learns over time.

## How Does The ML Piece Work?

SMAnalyzer uses a K-means clustering algorithm, which automatically sorts things into groups based on how similar they are to each other.

For example - imagine you have a bunch of different colored dots scattered on a piece of paper, and you want to organize them into groups where similar colors are together. K-means does this automatically.

The engine is designed to identify patterns in time series data by grouping similar behavioral segments based on statistical features.

## Why K-means?

K-means is used here because it's well-suited for baseline behavior pattern learning in service mesh environments.

  1. Automatic Pattern Discovery: K-means finds natural groupings in service
  behavior without requiring predefined categories. Services naturally exhibit
  different "behavioral modes" (normal load, peak traffic, maintenance periods,
  etc.)

  2. Baseline Establishment: The algorithm learns what "normal" looks like by
  clustering historical metric patterns. This creates behavioral baselines for
  anomaly detection.

  3. Multi-dimensional Analysis: Service mesh metrics have multiple dimensions
  (error rates, latency, throughput, etc.). K-means handles this
  multi-dimensional feature space effectively by clustering on the extracted
  features (mean, std dev, trend, volatility).

  4. Unsupervised Learning: No manual labeling of "good" vs "bad" behavior is
  needed. The algorithm discovers patterns automatically from the data.

  5. Computational Efficiency: K-means is fast enough for real-time monitoring
  scenarios where the system needs to continuously analyze incoming metrics.

  The clustering results help the anomaly detection engine distinguish between
  genuinely anomalous behavior versus normal variations in service performance
  patterns, reducing false positives in service mesh monitoring.

## Core Components
  1. CLI Framework (cmd/) - Cobra-based commands: scan, learn, monitor, status
  2. Kubernetes Client (pkg/k8s/) - Simple kubeconfig-based cluster connection
  3. Istio Discovery (pkg/istio/) - Service mesh metrics collection and service
  discovery
  4. Time Series Storage (pkg/timeseries/) - In-memory storage for metric data
  points
  5. ML Clustering (pkg/ml/) - K-means algorithm for behavior pattern learning
  6. Anomaly Detection (pkg/anomaly/) - Hybrid detection engine (rule-based + ML)
  7. Output Formatting (pkg/output/) - CLI-friendly output (text, table, JSON)
  8. Configuration (pkg/config/) - Centralized configuration management

## Key Features

- Multi-modal detection: Static thresholds + ML clustering for comprehensive
anomaly detection
- Service mesh focus: Specifically designed for Istio environments
- Learning capability: Establishes baseline behavior patterns through clustering
- Real-time monitoring: Continuous scanning with configurable intervals
- Multiple output formats: Human-readable and machine-parseable outputs
- Configurable thresholds: Adjustable sensitivity and detection parameters

## Usage

You'll see four use cases within the `smanalyzer` command:
1. Scan
2. Status

`cmd/scan.go`

  Implements the main scan command with flags for:
  - --namespace - target specific K8s namespace
  - --duration - how long to monitor
  - --learn - learning mode vs detection mode
  - Basic scan workflow placeholder

`cmd/status.go`

This command provides system status overview:

- status command: Quick health check and overview of the entire system
- Cluster connection: Shows if connected to Kubernetes and basic cluster info
- Service mesh status: Istio version, number of services with sidecars
- AI model status: Whether baseline is trained, when last updated, training
duration
- Recent activity: Anomaly counts over different time periods
- Configuration: Current detection thresholds and settings

`pkg/k8s/client.go`

  Simple Kubernetes client wrapper that uses the standard kubeconfig from the
  user's environment.

`pkg/istio/discovery.go`

  This file handles service mesh discovery and metrics collection:

  - ServiceDiscovery struct: Wraps the Kubernetes client to find Istio-enabled
  services
  - ServiceMeshMetrics struct: Defines the data structure for all metrics we care
  about (request counts, error rates, response times, circuit breaker status,
  retries, timeouts)
  - DiscoverServices(): Finds services with Istio sidecars by checking labels
  - CollectMetrics(): Gathers real-time metrics from Prometheus/Envoy (currently
  uses mock data)
  - hasIstioSidecar(): Helper to identify services that are part of the mesh

The core idea is: scan → discover services → collect metrics → analyze patterns → detect anomalies.

`pkg/timeseries/storage.go`

  This file provides in-memory time series data storage:

  - DataPoint struct: Single metric measurement with timestamp, value, and labels
  - TimeSeries struct: Collection of data points for a specific service/metric
  combination
  - Storage struct: Thread-safe storage managing multiple time series with mutex
  protection
  - Store(): Adds new data points to time series
  - GetSeries(): Retrieves a specific time series
  - GetTimeRange(): Gets data points within a time window for analysis
  - GetLatestN(): Gets the most recent N data points for real-time monitoring

`pkg/ml/clustering.go`

  This file implements machine learning clustering for behavior
   pattern analysis:

  - ClusterPoint struct: Wraps data points with extracted
  feature vectors
  - Cluster struct: Groups similar behavior patterns with
  centroids
  - KMeansConfig: Configuration for the K-means clustering
  algorithm
  - ExtractFeatures(): Converts time series data into feature
  vectors (mean, std dev, trend, volatility)
  - KMeans(): Core clustering algorithm that groups similar
  network behavior patterns
  - Statistical functions: Calculate mean, standard deviation,
  trend, and volatility from time windows
  - Distance calculations: Euclidean distance for clustering
  similarity measurements

This enables the system to learn "normal" traffic patterns and identify when services deviate from expected behavior.

`pkg/anomaly/detector.go`

  This file implements the core anomaly detection engine:

  - AnomalyType constants: Different types of service mesh issues (traffic spikes,
  high error rates, latency, circuit breaker trips, retry storms, timeouts)
  - Anomaly struct: Complete anomaly information including type, severity,
  description, metrics
  - DetectionConfig: Configurable thresholds and sensitivity settings
  - LearnBaseline(): Establishes normal behavior patterns using clustering
  - DetectAnomalies(): Two-pronged detection approach:
    - Static detection: Rule-based thresholds for obvious issues
    - ML detection: Compares current behavior against learned baseline clusters
  - Severity calculation: Quantifies how severe each anomaly is
  - Dynamic thresholds: Adapts sensitivity based on historical variance in the data

### Build Binary

```
go build .
```

### Run Commands

- smanalyzer scan - One-time anomaly scan
- smanalyzer status - System health and configuration overview


### Examples

```
./smanalyzer scan
                
Starting Service Mesh scan...
Scanning all namespaces
Duration: 5m0s
Learning mode: false
Connecting to Kubernetes cluster...
✓ Connected to Kubernetes cluster
Initializing Envoy metrics collection...
✓ Ready to collect metrics from Envoy sidecars
Discovering Services in Mesh...
Warning: Istio ingress gateway not found: deployments.apps "istio-ingressgateway" not found
✓ Istio control plane healthy (Pilot: 1 replicas)
Debug: DiscoverServices called with namespace=''
Debug: Searching in namespace=''
Debug: Found 39 total pods in namespace ''
Debug: Found Istio service: emoji-svc.emojivoto
Debug: Found Istio service: vote-bot.emojivoto
Debug: Found Istio service: voting-svc.emojivoto
Debug: Found Istio service: web-svc.emojivoto
✓ Found 4 services with Istio sidecars
Collecting service mesh metrics...
Debug: Collecting metrics for service vote-bot in namespace emojivoto
  Attempting to collect metrics from pod vote-bot-6fbc55bdb7-jlnhh
    📊 Metrics collected: Requests=1371736, RPS=22862.3, Errors=7.89%, P99=0s
```

```
./smanalyzer status
Service Mesh Analyzer Status
============================

🔍 Cluster Connection:
  Status: Connected
  Cluster: kind-kind
  Namespaces: 12

🕸️  Service Mesh:
  Istio Version: 1.20.0
  Services with sidecars: 15
  Gateway services: 2

🤖 AI Model:
  Baseline Status: Trained
  Last Updated: 2024-01-15 14:30:00
  Training Data: 24h

📊 Recent Activity:
  Anomalies (last 1h): 2
  Anomalies (last 24h): 12
  Services monitored: 15

⚙️  Configuration:
  Error rate threshold: 5%
  Traffic spike threshold: 2x
  Sensitivity level: 2.0
```