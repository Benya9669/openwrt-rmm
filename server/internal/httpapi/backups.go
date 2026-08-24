package httpapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	pathpkg "path"
	"strconv"
	"strings"
	"time"

	"rmm-openwrt/server/internal/store"
)

const maxDeviceBackupSize = 32 << 20

func (a *App) handleCreateDeviceBackup(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := backupDeviceID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	backup, command, found, err := a.store.RequestDeviceBackup(r.Context(), deviceID)
	if errors.Is(err, store.ErrDeviceBackupInProgress) {
		writeError(w, http.StatusConflict, "a device backup is already in progress")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backup command")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "backup.create", deviceID, command.ID, mustJSON(map[string]string{"backup_id": backup.ID, "request_id": requestID(r.Context())}))
	a.events.publish("devices")
	writeJSON(w, http.StatusAccepted, backup)
}

func (a *App) handleListDeviceBackups(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := backupDeviceID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	backups, found, err := a.store.ListDeviceBackups(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

func (a *App) handleDeleteDeviceBackup(w http.ResponseWriter, r *http.Request) {
	deviceID, backupID, ok := backupIDs(r.URL.Path, false)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deleted, err := a.store.DeleteDeviceBackup(r.Context(), deviceID, backupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "backup.delete", deviceID, "", mustJSON(map[string]string{"backup_id": backupID, "request_id": requestID(r.Context())}))
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRestoreDeviceBackup(w http.ResponseWriter, r *http.Request) {
	deviceID, backupID, ok := backupIDs(r.URL.Path, true)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	backup, found, err := a.store.GetDeviceBackup(r.Context(), deviceID, backupID, false)
	if err != nil || !found {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	if backup.Status != "ready" {
		writeError(w, http.StatusConflict, "backup is not ready")
		return
	}
	device, found, err := a.store.GetDevice(r.Context(), deviceID)
	if err != nil || !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	var inventory struct {
		Target string `json:"target"`
	}
	_ = json.Unmarshal(device.Inventory, &inventory)
	if strings.TrimSpace(backup.Target) == "" || strings.TrimSpace(inventory.Target) == "" || backup.Target != inventory.Target {
		writeError(w, http.StatusConflict, "backup target is not compatible with this device")
		return
	}
	args, _ := json.Marshal(map[string]string{"backup_id": backup.ID, "sha256": backup.SHA256, "target": backup.Target})
	command, found, err := a.store.CreateCommand(r.Context(), deviceID, "system_backup_restore", args)
	if err != nil || !found {
		writeError(w, http.StatusInternalServerError, "failed to create restore command")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "backup.restore", deviceID, command.ID, mustJSON(map[string]string{"backup_id": backup.ID, "request_id": requestID(r.Context())}))
	a.events.publish("devices")
	writeJSON(w, http.StatusAccepted, map[string]any{"backup": backup, "command": command})
}

func (a *App) handleAgentBackupUpload(w http.ResponseWriter, r *http.Request) {
	commandID := strings.TrimPrefix(r.URL.Path, "/api/agent/backups/")
	deviceID := strings.TrimSpace(r.Header.Get("X-RMM-Device-ID"))
	if !safePathID(commandID) || !safePathID(deviceID) {
		writeError(w, http.StatusBadRequest, "invalid backup identity")
		return
	}
	if err := a.authorizeDevice(r, deviceID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDeviceBackupSize)
	archive, err := io.ReadAll(r.Body)
	if err != nil || len(archive) == 0 {
		writeError(w, http.StatusBadRequest, "backup archive is empty or too large")
		return
	}
	manifest, err := backupManifest(archive)
	if err != nil {
		writeError(w, http.StatusBadRequest, "backup archive is invalid")
		return
	}
	manifestJSON, _ := json.Marshal(manifest)
	backup, found, err := a.store.SaveDeviceBackup(r.Context(), deviceID, commandID, archive, store.DeviceBackupMetadata{
		OpenWrtVersion: limitedHeader(r, "X-RMM-OpenWrt-Version", 512),
		Target:         limitedHeader(r, "X-RMM-Target", 255), Model: limitedHeader(r, "X-RMM-Model", 255), Manifest: manifestJSON,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store encrypted backup")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "backup command not found")
		return
	}
	writeJSON(w, http.StatusCreated, backup)
}

func (a *App) handleAgentBackupArchive(w http.ResponseWriter, r *http.Request) {
	backupID := strings.TrimPrefix(r.URL.Path, "/api/agent/backups/")
	deviceID := strings.TrimSpace(r.Header.Get("X-RMM-Device-ID"))
	if !safePathID(backupID) || !safePathID(deviceID) {
		writeError(w, http.StatusBadRequest, "invalid backup identity")
		return
	}
	if err := a.authorizeDevice(r, deviceID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	backup, archive, found, err := a.store.DeviceBackupArchive(r.Context(), deviceID, backupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt backup")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	w.Header().Set("X-RMM-SHA256", backup.SHA256)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func (a *App) handleDatabaseSnapshot(w http.ResponseWriter, r *http.Request) {
	path, err := a.store.CreateSQLiteSnapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create consistent database snapshot")
		return
	}
	defer os.Remove(path)
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open database snapshot")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to inspect database snapshot")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), actorName(r), "database.snapshot", "", "", mustJSON(map[string]string{"request_id": requestID(r.Context())}))
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="rmm-%s.db"`, time.Now().UTC().Format("20060102T150405Z")))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, file)
}

func backupManifest(archive []byte) ([]string, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	files := make([]string, 0)
	var expandedSize int64
	for len(files) < 4096 {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if header.Size < 0 || header.Size > (128<<20)-expandedSize {
			return nil, errors.New("backup archive expands beyond the safety limit")
		}
		expandedSize += header.Size
		name := strings.TrimSpace(header.Name)
		cleaned := pathpkg.Clean(name)
		if name == "" || strings.HasPrefix(name, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") || len(cleaned) > 1024 {
			return nil, errors.New("backup archive contains an unsafe path")
		}
		files = append(files, cleaned)
	}
	return nil, errors.New("backup archive has too many entries")
}

func limitedHeader(r *http.Request, name string, limit int) string {
	value := strings.TrimSpace(r.Header.Get(name))
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func safePathID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func backupDeviceID(path string) (string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "devices" || parts[3] != "backups" {
		return "", false
	}
	return parts[2], safePathID(parts[2])
}

func backupIDs(path string, restore bool) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	want := 5
	if restore {
		want = 6
	}
	ok := len(parts) == want && parts[0] == "api" && parts[1] == "devices" && parts[3] == "backups"
	if restore {
		ok = ok && parts[5] == "restore"
	}
	if !ok {
		return "", "", false
	}
	return parts[2], parts[4], safePathID(parts[2]) && safePathID(parts[4])
}
