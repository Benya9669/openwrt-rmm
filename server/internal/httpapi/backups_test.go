package httpapi_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"rmm-openwrt/internal/fieldcrypto"
	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func TestManagedBackupUploadRestoreAndDatabaseSnapshot(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenSQLite(ctx, filepath.Join(t.TempDir(), "managed-backup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cipher, _ := fieldcrypto.New(bytes.Repeat([]byte{6}, 32))
	if err := st.SetSensitiveDataCipher(cipher); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{EnrollmentToken: "enroll-backup", OperatorToken: "operator-backup"}))
	defer srv.Close()

	var enrolled struct {
		DeviceID    string `json:"device_id"`
		DeviceToken string `json:"device_token"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/enroll", "", map[string]any{
		"enrollment_token": "enroll-backup", "hostname": "backup-router", "openwrt_version": "25.12.4",
	}, http.StatusCreated, &enrolled)
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/heartbeat", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"inventory": map[string]any{"hostname": "backup-router", "target": "ramips-mt7621"},
		"metrics":   map[string]any{},
	}, http.StatusOK, nil)

	var requested model.DeviceBackup
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/backups", "operator-backup", nil, http.StatusAccepted, &requested)
	if requested.ID == "" || requested.CommandID == "" || requested.Status != "creating" {
		t.Fatalf("unexpected backup request: %#v", requested)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/backups", "operator-backup", nil, http.StatusConflict, nil)

	archive := testSysupgradeArchive(t)
	upload, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/agent/backups/"+requested.CommandID, bytes.NewReader(archive))
	upload.Header.Set("Authorization", "Bearer "+enrolled.DeviceToken)
	upload.Header.Set("X-RMM-Device-ID", enrolled.DeviceID)
	upload.Header.Set("X-RMM-OpenWrt-Version", "OpenWrt 25.12.4")
	upload.Header.Set("X-RMM-Target", "ramips-mt7621")
	upload.Header.Set("X-RMM-Model", "Test Router")
	uploadResponse, err := http.DefaultClient.Do(upload)
	if err != nil {
		t.Fatal(err)
	}
	defer uploadResponse.Body.Close()
	if uploadResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadResponse.Body)
		t.Fatalf("backup upload status=%d body=%s", uploadResponse.StatusCode, body)
	}

	var listed struct {
		Backups []model.DeviceBackup `json:"backups"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/backups", "operator-backup", nil, http.StatusOK, &listed)
	if len(listed.Backups) != 1 || listed.Backups[0].Status != "ready" || len(listed.Backups[0].Manifest) == 0 {
		t.Fatalf("unexpected backup list: %#v", listed.Backups)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/backups/"+requested.ID+"/restore", "operator-backup", nil, http.StatusAccepted, nil)

	download, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/agent/backups/"+requested.ID, nil)
	download.Header.Set("Authorization", "Bearer "+enrolled.DeviceToken)
	download.Header.Set("X-RMM-Device-ID", enrolled.DeviceID)
	downloadResponse, err := http.DefaultClient.Do(download)
	if err != nil {
		t.Fatal(err)
	}
	defer downloadResponse.Body.Close()
	downloaded, _ := io.ReadAll(downloadResponse.Body)
	if downloadResponse.StatusCode != http.StatusOK || !bytes.Equal(downloaded, archive) {
		t.Fatalf("downloaded backup mismatch: status=%d size=%d", downloadResponse.StatusCode, len(downloaded))
	}

	snapshot, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/admin/database-snapshot", nil)
	snapshot.Header.Set("Authorization", "Bearer operator-backup")
	snapshotResponse, err := http.DefaultClient.Do(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshotResponse.Body.Close()
	header := make([]byte, 16)
	_, _ = io.ReadFull(snapshotResponse.Body, header)
	if snapshotResponse.StatusCode != http.StatusOK || string(header) != "SQLite format 3\x00" {
		t.Fatalf("invalid database snapshot: status=%d header=%q", snapshotResponse.StatusCode, header)
	}
}

func testSysupgradeArchive(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	data := []byte("config interface 'lan'\n")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "etc/config/network", Mode: 0o600, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
