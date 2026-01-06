package metrics

import (
	"log"
	"sync/atomic"
	"time"
)

type Metrics struct {
	WritesTotal    atomic.Uint64
	ReadsTotal     atomic.Uint64
	LeaderChanges  atomic.Uint64
	WriteLatencyNS atomic.Uint64
}

var M = &Metrics{}

// RecordWrite records a write with latency
func RecordWrite(latency time.Duration) {
	M.WritesTotal.Add(1)
	M.WriteLatencyNS.Add(uint64(latency.Nanoseconds()))
}

// RecordRead records a read
func RecordRead() {
	M.ReadsTotal.Add(1)
}

// RecordLeaderChange increments leader change counter
func RecordLeaderChange() {
	M.LeaderChanges.Add(1)
}

// StartLogger periodically logs metrics
func StartLogger(interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()

		for range t.C {
			writes := M.WritesTotal.Load()
			reads := M.ReadsTotal.Load()
			leaders := M.LeaderChanges.Load()
			latNS := M.WriteLatencyNS.Load()

			avgLatency := float64(0)
			if writes > 0 {
				avgLatency = float64(latNS) / float64(writes) / 1e6
			}

			log.Printf(
				"[metrics] writes=%d reads=%d leader_changes=%d avg_write_latency_ms=%.2f",
				writes, reads, leaders, avgLatency,
			)
		}
	}()
}
