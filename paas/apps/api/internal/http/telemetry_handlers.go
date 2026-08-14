package http

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// Telemetry History & Logging Store

type TelemetryDataPoint struct {
	Timestamp          string  `json:"timestamp"`
	CPUPercent         float64 `json:"cpu_percent"`
	MemoryUsedPercent  float64 `json:"memory_used_percent"`
	MemoryUsedBytes    uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes   uint64  `json:"memory_total_bytes"`
	StorageUsedPercent float64 `json:"storage_used_percent"`
	StorageUsedBytes   uint64  `json:"storage_used_bytes"`
	StorageTotalBytes  uint64  `json:"storage_total_bytes"`
	Load1              float64 `json:"load1"`
	Load5              float64 `json:"load5"`
	Load15             float64 `json:"load15"`
	ActiveContainers   int     `json:"active_containers"`
}

type TelemetryLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

var (
	telemetryMu      sync.RWMutex
	telemetryHistory []TelemetryDataPoint
	telemetryLogs    []TelemetryLogEntry
)

func init() {
	// Initialize with baseline synthetic warmup points if empty
	now := time.Now().UTC()
	for i := 19; i >= 0; i-- {
		t := now.Add(-time.Duration(i*5) * time.Second)
		telemetryHistory = append(telemetryHistory, TelemetryDataPoint{
			Timestamp:          t.Format("15:04:05"),
			CPUPercent:         4.5 + float64(i%3)*1.2,
			MemoryUsedPercent:  28.0 + float64(i%4)*0.5,
			MemoryUsedBytes:    2147483648,
			MemoryTotalBytes:   8589934592,
			StorageUsedPercent: 18.5,
			StorageUsedBytes:   10737418240,
			StorageTotalBytes:  64424509440,
			Load1:              0.15,
			Load5:              0.12,
			Load15:             0.08,
			ActiveContainers:   2,
		})
	}
	telemetryLogs = append(telemetryLogs, TelemetryLogEntry{
		Timestamp: now.Format("15:04:05"),
		Level:     "system",
		Source:    "supervisor",
		Message:   "Telemetry engine initialized. Recording live capacity metrics and trend buffers.",
	})
}

func recordTelemetryPoint(point TelemetryDataPoint, logMsg string, logLevel string) {
	telemetryMu.Lock()
	defer telemetryMu.Unlock()

	telemetryHistory = append(telemetryHistory, point)
	if len(telemetryHistory) > 60 {
		telemetryHistory = telemetryHistory[len(telemetryHistory)-60:]
	}

	if logMsg != "" {
		telemetryLogs = append(telemetryLogs, TelemetryLogEntry{
			Timestamp: point.Timestamp,
			Level:     logLevel,
			Source:    "host-metrics",
			Message:   logMsg,
		})
		if len(telemetryLogs) > 100 {
			telemetryLogs = telemetryLogs[len(telemetryLogs)-100:]
		}
	}
}

func (h *Handler) handleGetTelemetry(c fiber.Ctx) error {
	vMem, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	cpuPercentages, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil {
		return err
	}
	avgLoad, _ := load.Avg()
	diskUsage, _ := disk.Usage("/")

	var cpuPct float64
	if len(cpuPercentages) > 0 {
		cpuPct = cpuPercentages[0]
	}

	// Count active docker containers
	containerCount := 0
	out, cErr := exec.Command("docker", "ps", "-q").CombinedOutput()
	if cErr == nil && len(out) > 0 {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				containerCount++
			}
		}
	}

	now := time.Now().UTC()
	point := TelemetryDataPoint{
		Timestamp:          now.Format("15:04:05"),
		CPUPercent:         cpuPct,
		MemoryUsedPercent:  vMem.UsedPercent,
		MemoryUsedBytes:    vMem.Used,
		MemoryTotalBytes:   vMem.Total,
		StorageUsedPercent: diskUsage.UsedPercent,
		StorageUsedBytes:   diskUsage.Used,
		StorageTotalBytes:  diskUsage.Total,
		Load1:              avgLoad.Load1,
		Load5:              avgLoad.Load5,
		Load15:             avgLoad.Load15,
		ActiveContainers:   containerCount,
	}

	logMsg := fmt.Sprintf("CPU: %.1f%% | Mem: %.1f GB/%.1f GB (%.1f%%) | Containers: %d active | Load1: %.2f",
		cpuPct,
		float64(vMem.Used)/1024/1024/1024,
		float64(vMem.Total)/1024/1024/1024,
		vMem.UsedPercent,
		containerCount,
		avgLoad.Load1,
	)
	logLevel := "info"
	if cpuPct > 80.0 || vMem.UsedPercent > 85.0 {
		logLevel = "warn"
	}

	recordTelemetryPoint(point, logMsg, logLevel)

	telemetryMu.RLock()
	trendsCopy := make([]TelemetryDataPoint, len(telemetryHistory))
	copy(trendsCopy, telemetryHistory)
	logsCopy := make([]TelemetryLogEntry, len(telemetryLogs))
	copy(logsCopy, telemetryLogs)
	telemetryMu.RUnlock()

	return c.JSON(fiber.Map{
		"host": fiber.Map{
			"cpu_percent":          cpuPct,
			"load1":                avgLoad.Load1,
			"load5":                avgLoad.Load5,
			"load15":               avgLoad.Load15,
			"memory_total_bytes":   vMem.Total,
			"memory_used_bytes":    vMem.Used,
			"memory_free_bytes":    vMem.Free,
			"memory_used_percent":  vMem.UsedPercent,
			"storage_total_bytes":  diskUsage.Total,
			"storage_used_bytes":   diskUsage.Used,
			"storage_free_bytes":   diskUsage.Free,
			"storage_used_percent": diskUsage.UsedPercent,
			"active_containers":    containerCount,
		},
		"trends": trendsCopy,
		"logs":   logsCopy,
	})
}
