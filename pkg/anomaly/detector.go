package anomaly

import (
	"fmt"
	"math"
	"time"
	"smanalyzer/pkg/ml"
	"smanalyzer/pkg/timeseries"
)

type AnomalyType string

const (
	TrafficSpike     AnomalyType = "traffic_spike"
	ErrorRateHigh    AnomalyType = "error_rate_high"
	LatencyAnomaly   AnomalyType = "latency_anomaly"
	CircuitBreaker   AnomalyType = "circuit_breaker"
	RetryStorm       AnomalyType = "retry_storm"
	TimeoutAnomaly   AnomalyType = "timeout_anomaly"
)

type Anomaly struct {
	Type        AnomalyType           `json:"type"`
	ServiceName string                `json:"service_name"`
	Namespace   string                `json:"namespace"`
	Severity    float64               `json:"severity"`
	Description string                `json:"description"`
	Timestamp   time.Time             `json:"timestamp"`
	Metrics     map[string]float64    `json:"metrics"`
	Labels      map[string]string     `json:"labels"`
}

type DetectionConfig struct {
	TrafficSpikeThreshold  float64
	ErrorRateThreshold     float64
	LatencyThreshold       time.Duration
	RetryThreshold         int64
	TimeoutThreshold       int64
	WindowSize            int
	SensitivityLevel      float64
}

type Detector struct {
	config          DetectionConfig
	clusteringEngine *ml.ClusteringEngine
	baselines       map[string][]ml.Cluster
}

func NewDetector(config DetectionConfig, clusteringEngine *ml.ClusteringEngine) *Detector {
	return &Detector{
		config:           config,
		clusteringEngine: clusteringEngine,
		baselines:        make(map[string][]ml.Cluster),
	}
}

func (d *Detector) LearnBaseline(serviceName string, points []timeseries.DataPoint) error {
	if len(points) < d.config.WindowSize {
		return fmt.Errorf("insufficient data points for baseline learning")
	}

	features := d.clusteringEngine.ExtractFeatures(points, d.config.WindowSize)
	clusters := d.clusteringEngine.KMeans(features)
	
	d.baselines[serviceName] = clusters
	
	return nil
}

func (d *Detector) DetectAnomalies(serviceName string, recentPoints []timeseries.DataPoint) ([]Anomaly, error) {
	var anomalies []Anomaly
	
	staticAnomalies := d.detectStaticAnomalies(serviceName, recentPoints)
	anomalies = append(anomalies, staticAnomalies...)
	
	if clusters, exists := d.baselines[serviceName]; exists {
		mlAnomalies := d.detectMLAnomalies(serviceName, recentPoints, clusters)
		anomalies = append(anomalies, mlAnomalies...)
	}
	
	return anomalies, nil
}

func (d *Detector) detectStaticAnomalies(serviceName string, points []timeseries.DataPoint) []Anomaly {
	var anomalies []Anomaly
	
	if len(points) < 2 {
		return anomalies
	}
	
	latest := points[len(points)-1]
	
	if d.isTrafficSpike(points) {
		anomalies = append(anomalies, Anomaly{
			Type:        TrafficSpike,
			ServiceName: serviceName,
			Severity:    d.calculateTrafficSpikeSeverity(points),
			Description: fmt.Sprintf("Traffic spike detected: %.2f requests", latest.Value),
			Timestamp:   latest.Timestamp,
			Metrics:     map[string]float64{"current_traffic": latest.Value},
		})
	}
	
	if d.isHighErrorRate(points) {
		anomalies = append(anomalies, Anomaly{
			Type:        ErrorRateHigh,
			ServiceName: serviceName,
			Severity:    latest.Value / d.config.ErrorRateThreshold,
			Description: fmt.Sprintf("High error rate: %.2f%%", latest.Value*100),
			Timestamp:   latest.Timestamp,
			Metrics:     map[string]float64{"error_rate": latest.Value},
		})
	}
	
	return anomalies
}

func (d *Detector) detectMLAnomalies(serviceName string, points []timeseries.DataPoint, baselines []ml.Cluster) []Anomaly {
	var anomalies []Anomaly
	
	if len(points) < d.config.WindowSize {
		return anomalies
	}
	
	features := d.clusteringEngine.ExtractFeatures(points, d.config.WindowSize)
	if len(features) == 0 {
		return anomalies
	}
	
	latest := features[len(features)-1]
	minDistance := math.Inf(1)
	
	for _, cluster := range baselines {
		distance := d.euclideanDistance(latest.Features, cluster.Centroid)
		if distance < minDistance {
			minDistance = distance
		}
	}
	
	threshold := d.calculateDynamicThreshold(baselines)
	if minDistance > threshold {
		severity := minDistance / threshold
		anomalies = append(anomalies, Anomaly{
			Type:        "behavioral_anomaly",
			ServiceName: serviceName,
			Severity:    severity,
			Description: fmt.Sprintf("Behavioral anomaly detected (distance: %.2f)", minDistance),
			Timestamp:   time.Now(),
			Metrics:     map[string]float64{"anomaly_distance": minDistance},
		})
	}
	
	return anomalies
}

func (d *Detector) isTrafficSpike(points []timeseries.DataPoint) bool {
	if len(points) < 3 {
		return false
	}
	
	recent := points[len(points)-3:]
	baseline := d.calculateMean(points[:len(points)-3])
	currentRate := d.calculateMean(recent)
	
	return currentRate > baseline*d.config.TrafficSpikeThreshold
}

func (d *Detector) isHighErrorRate(points []timeseries.DataPoint) bool {
	if len(points) == 0 {
		return false
	}
	
	latest := points[len(points)-1]
	return latest.Value > d.config.ErrorRateThreshold
}

func (d *Detector) calculateTrafficSpikeSeverity(points []timeseries.DataPoint) float64 {
	if len(points) < 3 {
		return 1.0
	}
	
	recent := points[len(points)-3:]
	baseline := d.calculateMean(points[:len(points)-3])
	currentRate := d.calculateMean(recent)
	
	if baseline == 0 {
		return 1.0
	}
	
	return currentRate / baseline
}

func (d *Detector) calculateMean(points []timeseries.DataPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, p := range points {
		sum += p.Value
	}
	return sum / float64(len(points))
}

func (d *Detector) euclideanDistance(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

func (d *Detector) calculateDynamicThreshold(clusters []ml.Cluster) float64 {
	if len(clusters) == 0 {
		return 1.0
	}
	
	totalVariance := 0.0
	totalPoints := 0
	
	for _, cluster := range clusters {
		if len(cluster.Points) == 0 {
			continue
		}
		
		variance := 0.0
		for _, point := range cluster.Points {
			distance := d.euclideanDistance(point.Features, cluster.Centroid)
			variance += distance * distance
		}
		variance /= float64(len(cluster.Points))
		
		totalVariance += variance * float64(len(cluster.Points))
		totalPoints += len(cluster.Points)
	}
	
	if totalPoints == 0 {
		return 1.0
	}
	
	avgVariance := totalVariance / float64(totalPoints)
	return math.Sqrt(avgVariance) * d.config.SensitivityLevel
}