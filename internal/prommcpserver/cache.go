package prommcpserver

import (
	"sync"
	"time"
)

const metricsCacheTTL = 300 * time.Second

// MetricsCache caches the metric name list for list_metrics.
type MetricsCache struct {
	mu        sync.Mutex
	data      []string
	timestamp time.Time
}

func (c *MetricsCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = nil
	c.timestamp = time.Time{}
}

func (c *MetricsCache) get(fetch func() ([]string, error)) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.data != nil && now.Sub(c.timestamp) < metricsCacheTTL {
		return c.data, nil
	}
	data, err := fetch()
	if err != nil {
		return nil, err
	}
	c.data = data
	c.timestamp = now
	return data, nil
}
