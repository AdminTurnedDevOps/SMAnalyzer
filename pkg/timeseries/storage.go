package timeseries

import (
	"sync"
	"time"
)

type DataPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     float64     `json:"value"`
	Labels    map[string]string `json:"labels"`
}

type TimeSeries struct {
	ServiceName string      `json:"service_name"`
	Metric      string      `json:"metric"`
	Points      []DataPoint `json:"points"`
	mutex       sync.RWMutex
}

type Storage struct {
	series map[string]*TimeSeries
	mutex  sync.RWMutex
}

func NewStorage() *Storage {
	return &Storage{
		series: make(map[string]*TimeSeries),
	}
}

func (s *Storage) Store(serviceName, metric string, value float64, labels map[string]string) {
	key := serviceName + ":" + metric
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if s.series[key] == nil {
		s.series[key] = &TimeSeries{
			ServiceName: serviceName,
			Metric:      metric,
			Points:      make([]DataPoint, 0),
		}
	}
	
	point := DataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}
	
	s.series[key].mutex.Lock()
	s.series[key].Points = append(s.series[key].Points, point)
	s.series[key].mutex.Unlock()
}

func (s *Storage) GetSeries(serviceName, metric string) (*TimeSeries, bool) {
	key := serviceName + ":" + metric
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	series, exists := s.series[key]
	return series, exists
}

func (s *Storage) GetTimeRange(serviceName, metric string, start, end time.Time) []DataPoint {
	series, exists := s.GetSeries(serviceName, metric)
	if !exists {
		return nil
	}
	
	series.mutex.RLock()
	defer series.mutex.RUnlock()
	
	var result []DataPoint
	for _, point := range series.Points {
		if point.Timestamp.After(start) && point.Timestamp.Before(end) {
			result = append(result, point)
		}
	}
	
	return result
}

func (s *Storage) GetLatestN(serviceName, metric string, n int) []DataPoint {
	series, exists := s.GetSeries(serviceName, metric)
	if !exists {
		return nil
	}
	
	series.mutex.RLock()
	defer series.mutex.RUnlock()
	
	points := series.Points
	if len(points) <= n {
		return points
	}
	
	return points[len(points)-n:]
}