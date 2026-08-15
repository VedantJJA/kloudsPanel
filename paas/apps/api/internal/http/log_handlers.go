package http

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ─── Log Storage & In-Memory / Disk Stream ───────────────────────────────────

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" | "stderr" | "build" | "system" | "runtime"
	Message   string `json:"message"`
}

var (
	logMu             sync.RWMutex
	serviceLatestLogs = make(map[string][]LogEntry)
)

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
		saveLogToDisk(serviceID, entry)
	}

	if depID != "" {
		logs := serviceLatestLogs[depID]
		if len(logs) > 500 {
			logs = logs[1:]
		}
		serviceLatestLogs[depID] = append(logs, entry)
		saveLogToDisk(depID, entry)
	}
}

func clearLogs(serviceID, depID string) {
	logMu.Lock()
	defer logMu.Unlock()
	if serviceID != "" {
		serviceLatestLogs[serviceID] = []LogEntry{}
		clearLogDisk(serviceID)
	}
	if depID != "" {
		serviceLatestLogs[depID] = []LogEntry{}
		clearLogDisk(depID)
	}
}

func saveLogToDisk(id string, entry LogEntry) {
	logDir := "/tmp/paas_logs"
	_ = os.MkdirAll(logDir, 0755)
	f, err := os.OpenFile(filepath.Join(logDir, id+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		b, _ := json.Marshal(entry)
		_, _ = f.WriteString(string(b) + "\n")
	}
}

func clearLogDisk(id string) {
	logDir := "/tmp/paas_logs"
	_ = os.Remove(filepath.Join(logDir, id+".jsonl"))
}

func loadLogsFromDisk(id string) []LogEntry {
	logDir := "/tmp/paas_logs"
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

// ─── Log Handlers ─────────────────────────────────────────────────────────────

func (h *Handler) handleGetLogs(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		id = c.Params("deployId")
	}

	logMu.RLock()
	entries, exists := serviceLatestLogs[id]
	logMu.RUnlock()

	if exists && len(entries) > 0 {
		return c.JSON(fiber.Map{"entries": entries})
	}

	// Try reading from disk for this ID directly
	if diskEntries := loadLogsFromDisk(id); len(diskEntries) > 0 {
		return c.JSON(fiber.Map{"entries": diskEntries})
	}

	// 1. Try resolving as a Deployment ID
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
		if diskEntries := loadLogsFromDisk(dep.ServiceID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}

		s, sErr := h.store.Services().GetByID(c.Context(), dep.ServiceID)
		if sErr == nil && s != nil {
			logMu.RLock()
			entries, exists = serviceLatestLogs[s.Slug]
			logMu.RUnlock()
			if exists && len(entries) > 0 {
				return c.JSON(fiber.Map{"entries": entries})
			}
			if diskEntries := loadLogsFromDisk(s.Slug); len(diskEntries) > 0 {
				return c.JSON(fiber.Map{"entries": diskEntries})
			}

			// Query live docker logs
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
	}

	// 2. Try resolving service to check by other identifier (slug or ID)
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
		if diskEntries := loadLogsFromDisk(s.ID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}
		if diskEntries := loadLogsFromDisk(s.Slug); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}

		// Fallback to query live docker logs from container
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
			Message:   "Deployment in progress. Initializing build worker...",
		},
	}})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }
