// Package metrics implements architecture-neutral host and container
// metrics collection. It works on both amd64 and arm64 without
// optional vendor libraries (no NVML, no vendor-specific cgroups tools).
package metrics

import (
	"context"
	"log/slog"
	"time"
)

// HostMetrics holds a single host metrics snapshot.
type HostMetrics struct {
	ObservedAt        time.Time
	CPUPercent        float64
	Load1             float64
	MemoryTotalBytes  int64
	MemoryUsedBytes   int64
	StorageTotalBytes int64
	StorageUsedBytes  int64
	NetworkRxBytes    int64
	NetworkTxBytes    int64
}

// ContainerMetrics holds per-container resource usage.
type ContainerMetrics struct {
	DeploymentID    string
	ObservedAt      time.Time
	CPUPercent      float64
	MemoryUsed      int64
	MemoryLimit     int64
	NetworkRxBytes  int64
	NetworkTxBytes  int64
	BlockReadBytes  int64
	BlockWriteBytes int64
}

// Collector samples host and container metrics at a configured interval.
type Collector struct {
	logger   *slog.Logger
	interval time.Duration
	// TODO Phase 4: docker client reference for container stats
}

// NewCollector creates a metrics collector.
func NewCollector(logger *slog.Logger, interval time.Duration) *Collector {
	return &Collector{
		logger:   logger,
		interval: interval,
	}
}

// Run starts the collection loop. It blocks until ctx is done.
func (c *Collector) Run(ctx context.Context, sink func(HostMetrics)) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	c.logger.Info("metrics collector started", "interval", c.interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m, err := c.collectHost()
			if err != nil {
				c.logger.Warn("host metrics error", "err", err)
				continue
			}
			sink(m)
		}
	}
}

// collectHost gathers host-level metrics using /proc (Linux) or syscalls.
func (c *Collector) collectHost() (HostMetrics, error) {
	// TODO Phase 4: implement using golang.org/x/sys/unix and /proc/stat
	// Current stub returns zeroed metrics
	return HostMetrics{
		ObservedAt:       time.Now().UTC(),
		CPUPercent:       0,
		MemoryTotalBytes: 0,
		MemoryUsedBytes:  0,
	}, nil
}
