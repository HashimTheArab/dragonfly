package world

import (
	"sync/atomic"
	"time"
)

const worldMetricsLogInterval = 10 * time.Second

type worldMetrics struct {
	execEnqueueCount    atomic.Uint64
	execEnqueueNanos    atomic.Uint64
	execEnqueueMaxNanos atomic.Uint64
	queueLenMax         atomic.Int64

	txCount    atomic.Uint64
	txNanos    atomic.Uint64
	txMaxNanos atomic.Uint64

	tickCount    atomic.Uint64
	tickNanos    atomic.Uint64
	tickMaxNanos atomic.Uint64

	tickEntityNanos    atomic.Uint64
	tickScheduledNanos atomic.Uint64
	tickRandomNanos    atomic.Uint64
	tickNeighbourNanos atomic.Uint64

	prefetchDropped atomic.Uint64
}

type worldMetricsSnapshot struct {
	execEnqueueCount    uint64
	execEnqueueNanos    uint64
	execEnqueueMaxNanos uint64
	queueLenMax         int64

	txCount    uint64
	txNanos    uint64
	txMaxNanos uint64

	tickCount    uint64
	tickNanos    uint64
	tickMaxNanos uint64

	tickEntityNanos    uint64
	tickScheduledNanos uint64
	tickRandomNanos    uint64
	tickNeighbourNanos uint64

	prefetchDropped uint64
}

func (m *worldMetrics) observeExecEnqueue(d time.Duration, queueLen int) {
	nanos := uint64(d)
	m.execEnqueueCount.Add(1)
	m.execEnqueueNanos.Add(nanos)
	updateMaxUint64(&m.execEnqueueMaxNanos, nanos)
	updateMaxInt64(&m.queueLenMax, int64(queueLen))
}

func (m *worldMetrics) observeTx(d time.Duration) {
	nanos := uint64(d)
	m.txCount.Add(1)
	m.txNanos.Add(nanos)
	updateMaxUint64(&m.txMaxNanos, nanos)
}

func (m *worldMetrics) observeTick(total, entity, scheduled, random, neighbour time.Duration) {
	m.tickCount.Add(1)
	m.tickNanos.Add(uint64(total))
	m.tickEntityNanos.Add(uint64(entity))
	m.tickScheduledNanos.Add(uint64(scheduled))
	m.tickRandomNanos.Add(uint64(random))
	m.tickNeighbourNanos.Add(uint64(neighbour))
	updateMaxUint64(&m.tickMaxNanos, uint64(total))
}

func (m *worldMetrics) observePrefetchDropped() {
	m.prefetchDropped.Add(1)
}

func (m *worldMetrics) snapshotAndReset() worldMetricsSnapshot {
	return worldMetricsSnapshot{
		execEnqueueCount:    m.execEnqueueCount.Swap(0),
		execEnqueueNanos:    m.execEnqueueNanos.Swap(0),
		execEnqueueMaxNanos: m.execEnqueueMaxNanos.Swap(0),
		queueLenMax:         m.queueLenMax.Swap(0),
		txCount:             m.txCount.Swap(0),
		txNanos:             m.txNanos.Swap(0),
		txMaxNanos:          m.txMaxNanos.Swap(0),
		tickCount:           m.tickCount.Swap(0),
		tickNanos:           m.tickNanos.Swap(0),
		tickMaxNanos:        m.tickMaxNanos.Swap(0),
		tickEntityNanos:     m.tickEntityNanos.Swap(0),
		tickScheduledNanos:  m.tickScheduledNanos.Swap(0),
		tickRandomNanos:     m.tickRandomNanos.Swap(0),
		tickNeighbourNanos:  m.tickNeighbourNanos.Swap(0),
		prefetchDropped:     m.prefetchDropped.Swap(0),
	}
}

func (w *World) metricsLoop() {
	defer w.running.Done()

	t := time.NewTicker(worldMetricsLogInterval)
	defer t.Stop()

	for {
		select {
		case <-w.closing:
			return
		case <-t.C:
			s := w.metrics.snapshotAndReset()
			if s.txCount == 0 && s.tickCount == 0 && s.prefetchDropped == 0 {
				continue
			}
			w.conf.Log.Debug(
				"world perf",
				"window", worldMetricsLogInterval.String(),
				"queue_max", s.queueLenMax,
				"enqueue_avg_ms", avgMillis(s.execEnqueueNanos, s.execEnqueueCount),
				"enqueue_max_ms", millisFromNanos(s.execEnqueueMaxNanos),
				"tx_count", s.txCount,
				"tx_avg_ms", avgMillis(s.txNanos, s.txCount),
				"tx_max_ms", millisFromNanos(s.txMaxNanos),
				"tick_count", s.tickCount,
				"tick_avg_ms", avgMillis(s.tickNanos, s.tickCount),
				"tick_max_ms", millisFromNanos(s.tickMaxNanos),
				"tick_entities_avg_ms", avgMillis(s.tickEntityNanos, s.tickCount),
				"tick_scheduled_avg_ms", avgMillis(s.tickScheduledNanos, s.tickCount),
				"tick_random_avg_ms", avgMillis(s.tickRandomNanos, s.tickCount),
				"tick_neighbour_avg_ms", avgMillis(s.tickNeighbourNanos, s.tickCount),
				"prefetch_dropped", s.prefetchDropped,
			)
		}
	}
}

func updateMaxUint64(dst *atomic.Uint64, value uint64) {
	for {
		current := dst.Load()
		if value <= current || dst.CompareAndSwap(current, value) {
			return
		}
	}
}

func updateMaxInt64(dst *atomic.Int64, value int64) {
	for {
		current := dst.Load()
		if value <= current || dst.CompareAndSwap(current, value) {
			return
		}
	}
}

func avgMillis(total, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return millisFromNanos(total) / float64(count)
}

func millisFromNanos(nanos uint64) float64 {
	return float64(nanos) / float64(time.Millisecond)
}
