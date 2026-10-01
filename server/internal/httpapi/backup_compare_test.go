package httpapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func manifestFixture(t *testing.T, entries []tar.Header) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gz)
	for _, entry := range entries {
		if err := writer.WriteHeader(&entry); err != nil {
			t.Fatal(err)
		}
		if entry.Typeflag == tar.TypeReg || entry.Typeflag == 0 {
			if _, err := writer.Write(bytes.Repeat([]byte("x"), int(entry.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestDetailedBackupManifestHashesAndRejectsUnsafeEntries(t *testing.T) {
	archive := manifestFixture(t, []tar.Header{{Name: "etc/config/system", Typeflag: tar.TypeReg, Size: 10, Mode: 0o600}, {Name: "etc/config", Typeflag: tar.TypeDir, Mode: 0o700}})
	files, err := detailedBackupManifest(archive)
	if err != nil || len(files) != 2 || files[1].SHA256 == "" || files[1].Size != 10 {
		t.Fatalf("manifest: %+v %v", files, err)
	}
	for _, entries := range [][]tar.Header{
		{{Name: "../escape", Typeflag: tar.TypeReg}},
		{{Name: "/etc/shadow", Typeflag: tar.TypeReg}},
		{{Name: "etc/config/system", Typeflag: tar.TypeReg}, {Name: "etc/config/./system", Typeflag: tar.TypeReg}},
		{{Name: "etc/config/link", Typeflag: tar.TypeSymlink, Linkname: "../../../escape"}},
		{{Name: "etc/config/device", Typeflag: tar.TypeChar}},
	} {
		if _, err = detailedBackupManifest(manifestFixture(t, entries)); err == nil {
			t.Fatalf("unsafe archive accepted: %+v", entries)
		}
	}
}
