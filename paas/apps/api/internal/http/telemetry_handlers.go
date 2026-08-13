package http

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// ─── Telemetry Handlers ───────────────────────────────────────────────────────

func (h *Handler) handleGetTelemetry(c fiber.Ctx) error {
	vMem, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	cpuPercentages, err := cpu.Percent(200*time.Millisecond, false)
	if err != nil {
		return err
	}
	avgLoad, _ := load.Avg()
	diskUsage, _ := disk.Usage("/")

	var cpuPct float64
	if len(cpuPercentages) > 0 {
		cpuPct = cpuPercentages[0]
	}

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
		},
		"cpu": fiber.Map{
			"percent": cpuPct,
			"load1":   avgLoad.Load1,
			"load5":   avgLoad.Load5,
			"load15":  avgLoad.Load15,
		},
		"memory": fiber.Map{
			"total":       vMem.Total,
			"used":        vMem.Used,
			"free":        vMem.Free,
			"usedPercent": vMem.UsedPercent,
		},
		"disk": fiber.Map{
			"total":       diskUsage.Total,
			"used":        diskUsage.Used,
			"free":        diskUsage.Free,
			"usedPercent": diskUsage.UsedPercent,
		},
	})
}
