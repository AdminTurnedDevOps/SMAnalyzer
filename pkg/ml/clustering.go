package ml

import (
	"math"
	"smanalyzer/pkg/timeseries"
)

type ClusterPoint struct {
	Features []float64
	Label    string
	Original *timeseries.DataPoint
}

type Cluster struct {
	Centroid []float64
	Points   []ClusterPoint
}

type KMeansConfig struct {
	K            int
	MaxIter      int
	Tolerance    float64
	Features     []string
}

type ClusteringEngine struct {
	config KMeansConfig
}

func NewClusteringEngine(config KMeansConfig) *ClusteringEngine {
	return &ClusteringEngine{config: config}
}

func (ce *ClusteringEngine) ExtractFeatures(points []timeseries.DataPoint, windowSize int) []ClusterPoint {
	var features []ClusterPoint
	
	for i := windowSize; i < len(points); i++ {
		window := points[i-windowSize : i]
		
		feature := ClusterPoint{
			Features: make([]float64, 0),
			Original: &points[i],
		}
		
		feature.Features = append(feature.Features, ce.calculateMean(window))
		feature.Features = append(feature.Features, ce.calculateStdDev(window))
		feature.Features = append(feature.Features, ce.calculateTrend(window))
		feature.Features = append(feature.Features, ce.calculateVolatility(window))
		
		features = append(features, feature)
	}
	
	return features
}

func (ce *ClusteringEngine) KMeans(points []ClusterPoint) []Cluster {
	if len(points) < ce.config.K {
		return nil
	}
	
	clusters := ce.initializeClusters(points)
	
	for iter := 0; iter < ce.config.MaxIter; iter++ {
		oldCentroids := ce.copyCentroids(clusters)
		
		ce.assignPointsToClusters(points, clusters)
		
		ce.updateCentroids(clusters)
		
		if ce.hasConverged(oldCentroids, clusters) {
			break
		}
	}
	
	return clusters
}

func (ce *ClusteringEngine) calculateMean(points []timeseries.DataPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, p := range points {
		sum += p.Value
	}
	return sum / float64(len(points))
}

func (ce *ClusteringEngine) calculateStdDev(points []timeseries.DataPoint) float64 {
	if len(points) <= 1 {
		return 0
	}
	
	mean := ce.calculateMean(points)
	sumSquaredDiff := 0.0
	
	for _, p := range points {
		diff := p.Value - mean
		sumSquaredDiff += diff * diff
	}
	
	return math.Sqrt(sumSquaredDiff / float64(len(points)-1))
}

func (ce *ClusteringEngine) calculateTrend(points []timeseries.DataPoint) float64 {
	if len(points) < 2 {
		return 0
	}
	
	first := points[0].Value
	last := points[len(points)-1].Value
	
	return (last - first) / first
}

func (ce *ClusteringEngine) calculateVolatility(points []timeseries.DataPoint) float64 {
	if len(points) < 2 {
		return 0
	}
	
	changes := make([]float64, len(points)-1)
	for i := 1; i < len(points); i++ {
		if points[i-1].Value != 0 {
			changes[i-1] = (points[i].Value - points[i-1].Value) / points[i-1].Value
		}
	}
	
	mean := 0.0
	for _, change := range changes {
		mean += change
	}
	mean /= float64(len(changes))
	
	variance := 0.0
	for _, change := range changes {
		diff := change - mean
		variance += diff * diff
	}
	variance /= float64(len(changes))
	
	return math.Sqrt(variance)
}

func (ce *ClusteringEngine) initializeClusters(points []ClusterPoint) []Cluster {
	clusters := make([]Cluster, ce.config.K)
	
	for i := 0; i < ce.config.K; i++ {
		pointIdx := i * len(points) / ce.config.K
		clusters[i].Centroid = make([]float64, len(points[pointIdx].Features))
		copy(clusters[i].Centroid, points[pointIdx].Features)
		clusters[i].Points = make([]ClusterPoint, 0)
	}
	
	return clusters
}

func (ce *ClusteringEngine) assignPointsToClusters(points []ClusterPoint, clusters []Cluster) {
	for i := range clusters {
		clusters[i].Points = clusters[i].Points[:0]
	}
	
	for _, point := range points {
		minDist := math.Inf(1)
		clusterIdx := 0
		
		for i, cluster := range clusters {
			dist := ce.euclideanDistance(point.Features, cluster.Centroid)
			if dist < minDist {
				minDist = dist
				clusterIdx = i
			}
		}
		
		clusters[clusterIdx].Points = append(clusters[clusterIdx].Points, point)
	}
}

func (ce *ClusteringEngine) updateCentroids(clusters []Cluster) {
	for i := range clusters {
		if len(clusters[i].Points) == 0 {
			continue
		}
		
		for j := range clusters[i].Centroid {
			sum := 0.0
			for _, point := range clusters[i].Points {
				sum += point.Features[j]
			}
			clusters[i].Centroid[j] = sum / float64(len(clusters[i].Points))
		}
	}
}

func (ce *ClusteringEngine) euclideanDistance(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

func (ce *ClusteringEngine) copyCentroids(clusters []Cluster) [][]float64 {
	centroids := make([][]float64, len(clusters))
	for i, cluster := range clusters {
		centroids[i] = make([]float64, len(cluster.Centroid))
		copy(centroids[i], cluster.Centroid)
	}
	return centroids
}

func (ce *ClusteringEngine) hasConverged(oldCentroids [][]float64, clusters []Cluster) bool {
	for i, cluster := range clusters {
		dist := ce.euclideanDistance(oldCentroids[i], cluster.Centroid)
		if dist > ce.config.Tolerance {
			return false
		}
	}
	return true
}