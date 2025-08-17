package timeseries

import (
	"testing"
	"time"
)

func TestStorage_Store(t *testing.T) {
	storage := NewStorage()
	
	labels := map[string]string{"env": "test"}
	storage.Store("test-service", "request_count", 100.0, labels)
	
	series, exists := storage.GetSeries("test-service", "request_count")
	if !exists {
		t.Fatal("Expected series to exist")
	}
	
	if series.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", series.ServiceName)
	}
	
	if series.Metric != "request_count" {
		t.Errorf("Expected metric 'request_count', got '%s'", series.Metric)
	}
	
	if len(series.Points) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(series.Points))
	}
	
	point := series.Points[0]
	if point.Value != 100.0 {
		t.Errorf("Expected value 100.0, got %f", point.Value)
	}
	
	if point.Labels["env"] != "test" {
		t.Errorf("Expected label env=test, got %s", point.Labels["env"])
	}
}

func TestStorage_GetSeries_NotExists(t *testing.T) {
	storage := NewStorage()
	
	_, exists := storage.GetSeries("nonexistent", "metric")
	if exists {
		t.Error("Expected series to not exist")
	}
}

func TestStorage_GetTimeRange(t *testing.T) {
	storage := NewStorage()
	
	now := time.Now()
	labels := map[string]string{}
	
	storage.series["test:metric"] = &TimeSeries{
		ServiceName: "test",
		Metric:      "metric",
		Points: []DataPoint{
			{Timestamp: now.Add(-2 * time.Hour), Value: 1.0, Labels: labels},
			{Timestamp: now.Add(-1 * time.Hour), Value: 2.0, Labels: labels},
			{Timestamp: now, Value: 3.0, Labels: labels},
			{Timestamp: now.Add(1 * time.Hour), Value: 4.0, Labels: labels},
		},
	}
	
	start := now.Add(-90 * time.Minute)
	end := now.Add(30 * time.Minute)
	
	points := storage.GetTimeRange("test", "metric", start, end)
	
	if len(points) != 2 {
		t.Errorf("Expected 2 points in range, got %d", len(points))
	}
	
	if points[0].Value != 2.0 {
		t.Errorf("Expected first point value 2.0, got %f", points[0].Value)
	}
	
	if points[1].Value != 3.0 {
		t.Errorf("Expected second point value 3.0, got %f", points[1].Value)
	}
}

func TestStorage_GetLatestN(t *testing.T) {
	storage := NewStorage()
	
	labels := map[string]string{}
	for i := 0; i < 10; i++ {
		storage.Store("test-service", "metric", float64(i), labels)
	}
	
	latest := storage.GetLatestN("test-service", "metric", 3)
	
	if len(latest) != 3 {
		t.Errorf("Expected 3 latest points, got %d", len(latest))
	}
	
	if latest[0].Value != 7.0 {
		t.Errorf("Expected first point value 7.0, got %f", latest[0].Value)
	}
	
	if latest[2].Value != 9.0 {
		t.Errorf("Expected last point value 9.0, got %f", latest[2].Value)
	}
}

func TestStorage_GetLatestN_MoreThanExists(t *testing.T) {
	storage := NewStorage()
	
	labels := map[string]string{}
	storage.Store("test", "metric", 1.0, labels)
	storage.Store("test", "metric", 2.0, labels)
	
	latest := storage.GetLatestN("test", "metric", 5)
	
	if len(latest) != 2 {
		t.Errorf("Expected 2 points when requesting 5, got %d", len(latest))
	}
}

func TestStorage_ConcurrentAccess(t *testing.T) {
	storage := NewStorage()
	
	done := make(chan bool, 2)
	
	go func() {
		for i := 0; i < 100; i++ {
			storage.Store("service1", "metric1", float64(i), map[string]string{})
		}
		done <- true
	}()
	
	go func() {
		for i := 0; i < 100; i++ {
			storage.Store("service2", "metric2", float64(i), map[string]string{})
		}
		done <- true
	}()
	
	<-done
	<-done
	
	series1, exists1 := storage.GetSeries("service1", "metric1")
	if !exists1 || len(series1.Points) != 100 {
		t.Errorf("Expected 100 points for service1, got %d", len(series1.Points))
	}
	
	series2, exists2 := storage.GetSeries("service2", "metric2")
	if !exists2 || len(series2.Points) != 100 {
		t.Errorf("Expected 100 points for service2, got %d", len(series2.Points))
	}
}