package ml

import (
	"math"
	"testing"
	"time"
	"smanalyzer/pkg/timeseries"
)

func TestClusteringEngine_ExtractFeatures(t *testing.T) {
	config := KMeansConfig{
		K:         3,
		MaxIter:   100,
		Tolerance: 0.01,
	}
	engine := NewClusteringEngine(config)
	
	points := []timeseries.DataPoint{
		{Timestamp: time.Now(), Value: 10.0},
		{Timestamp: time.Now(), Value: 12.0},
		{Timestamp: time.Now(), Value: 11.0},
		{Timestamp: time.Now(), Value: 13.0},
		{Timestamp: time.Now(), Value: 14.0},
		{Timestamp: time.Now(), Value: 15.0},
	}
	
	features := engine.ExtractFeatures(points, 3)
	
	expectedFeatures := len(points) - 3
	if len(features) != expectedFeatures {
		t.Errorf("Expected %d features, got %d", expectedFeatures, len(features))
	}
	
	if len(features[0].Features) != 4 {
		t.Errorf("Expected 4 feature dimensions, got %d", len(features[0].Features))
	}
	
	feature := features[0]
	expectedMean := (10.0 + 12.0 + 11.0) / 3.0
	if math.Abs(feature.Features[0]-expectedMean) > 0.001 {
		t.Errorf("Expected mean %.3f, got %.3f", expectedMean, feature.Features[0])
	}
}

func TestClusteringEngine_CalculateMean(t *testing.T) {
	engine := &ClusteringEngine{}
	
	points := []timeseries.DataPoint{
		{Value: 10.0},
		{Value: 20.0},
		{Value: 30.0},
	}
	
	mean := engine.calculateMean(points)
	expected := 20.0
	
	if math.Abs(mean-expected) > 0.001 {
		t.Errorf("Expected mean %.3f, got %.3f", expected, mean)
	}
}

func TestClusteringEngine_CalculateMean_Empty(t *testing.T) {
	engine := &ClusteringEngine{}
	
	points := []timeseries.DataPoint{}
	mean := engine.calculateMean(points)
	
	if mean != 0.0 {
		t.Errorf("Expected mean 0.0 for empty points, got %.3f", mean)
	}
}

func TestClusteringEngine_CalculateStdDev(t *testing.T) {
	engine := &ClusteringEngine{}
	
	points := []timeseries.DataPoint{
		{Value: 10.0},
		{Value: 12.0},
		{Value: 14.0},
		{Value: 16.0},
		{Value: 18.0},
	}
	
	stddev := engine.calculateStdDev(points)
	
	if stddev <= 0 {
		t.Error("Standard deviation should be positive")
	}
	
	if stddev > 10 {
		t.Errorf("Standard deviation seems too high: %.3f", stddev)
	}
}

func TestClusteringEngine_CalculateTrend(t *testing.T) {
	engine := &ClusteringEngine{}
	
	points := []timeseries.DataPoint{
		{Value: 10.0},
		{Value: 15.0},
		{Value: 20.0},
	}
	
	trend := engine.calculateTrend(points)
	expected := (20.0 - 10.0) / 10.0
	
	if math.Abs(trend-expected) > 0.001 {
		t.Errorf("Expected trend %.3f, got %.3f", expected, trend)
	}
}

func TestClusteringEngine_CalculateVolatility(t *testing.T) {
	engine := &ClusteringEngine{}
	
	points := []timeseries.DataPoint{
		{Value: 100.0},
		{Value: 105.0},
		{Value: 95.0},
		{Value: 110.0},
	}
	
	volatility := engine.calculateVolatility(points)
	
	if volatility <= 0 {
		t.Error("Volatility should be positive for varying values")
	}
}

func TestClusteringEngine_KMeans(t *testing.T) {
	config := KMeansConfig{
		K:         2,
		MaxIter:   10,
		Tolerance: 0.1,
	}
	engine := NewClusteringEngine(config)
	
	points := []ClusterPoint{
		{Features: []float64{1.0, 1.0}},
		{Features: []float64{1.5, 1.5}},
		{Features: []float64{2.0, 2.0}},
		{Features: []float64{10.0, 10.0}},
		{Features: []float64{10.5, 10.5}},
		{Features: []float64{11.0, 11.0}},
	}
	
	clusters := engine.KMeans(points)
	
	if len(clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(clusters))
	}
	
	totalPoints := 0
	for _, cluster := range clusters {
		totalPoints += len(cluster.Points)
		if len(cluster.Centroid) != 2 {
			t.Errorf("Expected centroid to have 2 dimensions, got %d", len(cluster.Centroid))
		}
	}
	
	if totalPoints != len(points) {
		t.Errorf("Expected all points to be assigned to clusters, got %d/%d", totalPoints, len(points))
	}
}

func TestClusteringEngine_KMeans_InsufficientPoints(t *testing.T) {
	config := KMeansConfig{
		K:         5,
		MaxIter:   10,
		Tolerance: 0.1,
	}
	engine := NewClusteringEngine(config)
	
	points := []ClusterPoint{
		{Features: []float64{1.0, 1.0}},
		{Features: []float64{2.0, 2.0}},
	}
	
	clusters := engine.KMeans(points)
	
	if clusters != nil {
		t.Error("Expected nil clusters when K > number of points")
	}
}

func TestClusteringEngine_EuclideanDistance(t *testing.T) {
	engine := &ClusteringEngine{}
	
	a := []float64{0.0, 0.0}
	b := []float64{3.0, 4.0}
	
	distance := engine.euclideanDistance(a, b)
	expected := 5.0
	
	if math.Abs(distance-expected) > 0.001 {
		t.Errorf("Expected distance %.3f, got %.3f", expected, distance)
	}
}

func TestClusteringEngine_HasConverged(t *testing.T) {
	config := KMeansConfig{Tolerance: 0.1}
	engine := NewClusteringEngine(config)
	
	oldCentroids := [][]float64{
		{1.0, 1.0},
		{2.0, 2.0},
	}
	
	clusters := []Cluster{
		{Centroid: []float64{1.05, 1.05}},
		{Centroid: []float64{2.05, 2.05}},
	}
	
	converged := engine.hasConverged(oldCentroids, clusters)
	if converged {
		t.Error("Expected not converged with changes > tolerance")
	}
	
	clusters[0].Centroid = []float64{1.01, 1.01}
	clusters[1].Centroid = []float64{2.01, 2.01}
	
	converged = engine.hasConverged(oldCentroids, clusters)
	if !converged {
		t.Error("Expected converged with changes < tolerance")
	}
}