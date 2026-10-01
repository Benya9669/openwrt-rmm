package httpapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
)

type backupFile struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

func detailedBackupManifest(archive []byte) ([]backupFile, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	files := []backupFile{}
	seen := map[string]bool{}
	var total int64
	for len(files) < 4096 {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			// Drain the bounded gzip tail so checksum/truncation errors cannot hide
			// behind tar's end-of-archive marker.
			remaining := (128 << 20) - total + (4 << 20)
			n, tailErr := io.Copy(io.Discard, io.LimitReader(gzipReader, remaining+1))
			if tailErr != nil {
				return nil, tailErr
			}
			if n > remaining {
				return nil, errors.New("backup exceeds expanded size limit")
			}
			sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(header.Name)
		cleaned := path.Clean(name)
		if name == "" || strings.HasPrefix(name, "/") || cleaned == ".." || (cleaned == "." && header.Typeflag != tar.TypeDir) || strings.HasPrefix(cleaned, "../") || len(cleaned) > 1024 || seen[cleaned] {
			return nil, errors.New("backup contains unsafe or duplicate paths")
		}
		seen[cleaned] = true
		if header.Size < 0 || header.Size > (128<<20)-total {
			return nil, errors.New("backup exceeds expanded size limit")
		}
		total += header.Size
		file := backupFile{Path: cleaned, Size: header.Size, Type: "file"}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			hash := sha256.New()
			if n, err := io.Copy(hash, io.LimitReader(tarReader, header.Size)); err != nil || n != header.Size {
				if err == nil {
					err = io.ErrUnexpectedEOF
				}
				return nil, err
			}
			file.SHA256 = hex.EncodeToString(hash.Sum(nil))
		case tar.TypeDir:
			file.Type = "directory"
		case tar.TypeSymlink, tar.TypeLink:
			link := path.Clean(path.Join(path.Dir(cleaned), header.Linkname))
			if header.Typeflag == tar.TypeLink {
				link = path.Clean(header.Linkname)
			}
			if strings.HasPrefix(header.Linkname, "/") || link == ".." || strings.HasPrefix(link, "../") {
				return nil, errors.New("backup contains unsafe link")
			}
			file.Type = "symlink"
			if header.Typeflag == tar.TypeLink {
				file.Type = "hardlink"
			}
			digest := sha256.Sum256([]byte(header.Linkname))
			file.SHA256 = hex.EncodeToString(digest[:])
		default:
			return nil, errors.New("backup contains unsupported entry type")
		}
		files = append(files, file)
	}
	return nil, errors.New("backup has too many entries")
}

func (a *App) handleCompareBackups(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 6 || !safePathID(r.URL.Query().Get("against")) {
		writeError(w, http.StatusBadRequest, "comparison backup is required")
		return
	}
	deviceID, backupID := parts[2], parts[4]
	_, first, found, err := a.store.DeviceBackupArchive(r.Context(), deviceID, backupID)
	if err != nil || !found {
		writeError(w, http.StatusNotFound, "backup archive not found")
		return
	}
	_, second, found, err := a.store.DeviceBackupArchive(r.Context(), deviceID, r.URL.Query().Get("against"))
	if err != nil || !found {
		writeError(w, http.StatusNotFound, "comparison backup archive not found")
		return
	}
	before, err := detailedBackupManifest(first)
	if err != nil {
		writeError(w, http.StatusConflict, "backup cannot be compared safely")
		return
	}
	after, err := detailedBackupManifest(second)
	if err != nil {
		writeError(w, http.StatusConflict, "comparison backup cannot be compared safely")
		return
	}
	old, newFiles := map[string]backupFile{}, map[string]backupFile{}
	for _, file := range before {
		old[file.Path] = file
	}
	for _, file := range after {
		newFiles[file.Path] = file
	}
	changes := []map[string]any{}
	for name, file := range old {
		if current, ok := newFiles[name]; !ok {
			changes = append(changes, map[string]any{"path": name, "change": "removed", "before_size": file.Size})
		} else if current.SHA256 != file.SHA256 || current.Size != file.Size || current.Type != file.Type {
			changes = append(changes, map[string]any{"path": name, "change": "changed", "before_size": file.Size, "after_size": current.Size})
		}
	}
	for name, file := range newFiles {
		if _, ok := old[name]; !ok {
			changes = append(changes, map[string]any{"path": name, "change": "added", "after_size": file.Size})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i]["path"].(string) < changes[j]["path"].(string) })
	writeJSON(w, http.StatusOK, map[string]any{"before_id": backupID, "after_id": r.URL.Query().Get("against"), "changes": changes, "unchanged": len(before) - countChangedOrRemoved(changes)})
}

func countChangedOrRemoved(changes []map[string]any) int {
	n := 0
	for _, change := range changes {
		if change["change"] != "added" {
			n++
		}
	}
	return n
}
