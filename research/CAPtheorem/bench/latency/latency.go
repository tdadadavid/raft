package main

import (
	"sync"
	"time"
)

type Histogram struct {
	lock sync.Mutex
	values []time.Duration
}

func NewHistogram(min, max time.Duration, sig int) *Histogram {
	return &Histogram{values: make([]time.Duration, 0, 100000)}
}

func (h *Histogram) RecordValue(d time.Duration) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.values = append(h.values, d)
}

func (h *Histogram) ValueAtQuantile(p int) time.Duration {
	h.lock.Lock()
	defer h.lock.Unlock()
	if len(h.values) == 0 {
		return 0
	}
	// simple sort-based approximate
	sorted := make([]time.Duration, len(h.values))
	copy(sorted, h.values)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	idx := int(float64(p)/100*float64(len(sorted)-1))
	return sorted[idx]
}

func (h *Histogram) Max() time.Duration {
	h.lock.Lock()
	defer h.lock.Unlock()
	var max time.Duration
	for _, v := range h.values {
		if v > max {
			max = v
		}
	}
	return max
}