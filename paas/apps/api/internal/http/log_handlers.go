package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/repository"
)

// --- Log Storage & In-Memory / Disk Stream -----------------------------------

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" | "stderr" | "build" | "system" | "runtime"
	Message   string `json:"message"`
}

var (
	logMu             sync.RWMutex
	serviceLatestLogs = make(map[string][]LogEntry)
)

func getLogsDir() string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "klouds_logs")
	}
	logsDir := filepath.Join(dataDir, "deployments")
	_ = os.MkdirAll(logsDir, 0755)
	return logsDir
}

func appendLog(serviceID, depID, stream, message string) {
	logMu.Lock()
	defer logMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format("15:04:05"),
		Stream:    stream,
		Message:   message,
	}

	if serviceID != "" {
		logs := serviceLatestLogs[serviceID]
		if len(logs) > 500 {
			logs = logs[1:]
		}
		serviceLatestLogs[serviceID] = append(logs, entry)
		saveLogToDisk("svc_"+serviceID, entry)
	}

	if depID != "" {
		logs := serviceLatestLogs[depID]
		if len(logs) > 1000 {
			logs = logs[1:]
		}
		serviceLatestLogs[depID] = append(logs, entry)
		saveLogToDisk(depID, entry)
	}
}

// pruneOldDeploymentLogs retains up to keepCount (e.g. 25) deployment log files on disk per service,
// while keeping all deployment metadata in the database without any limits.
func pruneOldDeploymentLogs(ctx context.Context, serviceID string, store repository.Store, keepCount int) {
	if store == nil || serviceID == "" || keepCount <= 0 {
		return
	}
	deps, err := store.Deployments().ListForService(ctx, serviceID, 500, nil)
	if err != nil || len(deps) <= keepCount {
		return
	}
	// deps are ordered by sequence DESC (newest first)
	for i := keepCount; i < len(deps); i++ {
		clearLogDisk(deps[i].ID)
	}
}

func saveLogToDisk(id string, entry LogEntry) {
	if id == "" {
		return
	}
	logDir := getLogsDir()
	filePath := filepath.Join(logDir, id+".jsonl")
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		b, _ := json.Marshal(entry)
		_, _ = f.WriteString(string(b) + "\n")
	}
}

func clearLogDisk(id string) {
	if id == "" {
		return
	}
	logDir := getLogsDir()
	_ = os.Remove(filepath.Join(logDir, id+".jsonl"))
}

func loadLogsFromDisk(id string) []LogEntry {
	if id == "" {
		return nil
	}
	logDir := getLogsDir()
	filePath := filepath.Join(logDir, id+".jsonl")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var entries []LogEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			var e LogEntry
			if err := json.Unmarshal([]byte(line), &e); err == nil {
				entries = append(entries, e)
			}
		}
	}
	return entries
}

// --- Log Handlers -------------------------------------------------------------

func (h *Handler) handleGetLogs(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		id = c.Params("deployId")
	}
	deployID := c.Query("deployId")

	// 1. If explicit deployId is requested (e.g. from history viewer)
	if deployID != "" {
		logMu.RLock()
		entries, exists := serviceLatestLogs[deployID]
		logMu.RUnlock()
		if exists && len(entries) > 0 {
			return c.JSON(fiber.Map{"entries": entries, "deployment_id": deployID})
		}
		if diskEntries := loadLogsFromDisk(deployID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries, "deployment_id": deployID})
		}
	}

	// 2. Check in-memory logs for this ID directly
	logMu.RLock()
	entries, exists := serviceLatestLogs[id]
	logMu.RUnlock()

	if exists && len(entries) > 0 {
		return c.JSON(fiber.Map{"entries": entries})
	}

	// 3. Try reading deployment log from disk
	if diskEntries := loadLogsFromDisk(id); len(diskEntries) > 0 {
		return c.JSON(fiber.Map{"entries": diskEntries})
	}
	// 4. Try reading service live log from disk
	if diskEntries := loadLogsFromDisk("svc_" + id); len(diskEntries) > 0 {
		return c.JSON(fiber.Map{"entries": diskEntries})
	}

	// 5. Try resolving as a Deployment record in DB
	dep, err := h.store.Deployments().GetByID(c.Context(), id)
	if err == nil && dep != nil {
		logMu.RLock()
		entries, exists = serviceLatestLogs[dep.ID]
		if !exists || len(entries) == 0 {
			entries, exists = serviceLatestLogs[dep.ServiceID]
		}
		logMu.RUnlock()
		if exists && len(entries) > 0 {
			return c.JSON(fiber.Map{"entries": entries})
		}

		if diskEntries := loadLogsFromDisk(dep.ID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}
		if diskEntries := loadLogsFromDisk("svc_" + dep.ServiceID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}
	}

	// 6. Try resolving as a Service record in DB
	s, err := h.store.Services().GetByID(c.Context(), id)
	if err == nil && s != nil {
		logMu.RLock()
		entries, exists = serviceLatestLogs[s.ID]
		if !exists || len(entries) == 0 {
			entries, exists = serviceLatestLogs[s.Slug]
		}
		logMu.RUnlock()
		if exists && len(entries) > 0 {
			return c.JSON(fiber.Map{"entries": entries})
		}
		if diskEntries := loadLogsFromDisk("svc_" + s.ID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}
		if diskEntries := loadLogsFromDisk("svc_" + s.Slug); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}

		// Check latest deployment for this service
		deps, dErr := h.store.Deployments().ListForService(c.Context(), s.ID, 1, nil)
		if dErr == nil && len(deps) > 0 {
			if diskEntries := loadLogsFromDisk(deps[0].ID); len(diskEntries) > 0 {
				return c.JSON(fiber.Map{"entries": diskEntries})
			}
		}

		// Fallback: Query live docker logs from container
		containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
		cmd := exec.Command("docker", "logs", "--tail", "150", containerName)
		out, err := cmd.CombinedOutput()
		if err == nil && len(out) > 0 {
			var liveEntries []LogEntry
			now := time.Now().UTC()
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) != "" {
					liveEntries = append(liveEntries, LogEntry{
						Timestamp: now.Format("15:04:05"),
						Stream:    "stdout",
						Message:   line,
					})
				}
			}
			if len(liveEntries) > 0 {
				return c.JSON(fiber.Map{"entries": liveEntries})
			}
		}
	}

	return c.JSON(fiber.Map{"entries": []LogEntry{
		{
			Timestamp: time.Now().UTC().Format("15:04:05"),
			Stream:    "system",
			Message:   "No active logs recorded. Deployment logs will stream here during builds.",
		},
	}})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }
