package http

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractArchiveZip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paas-zip-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "test.zip")
	destDir := filepath.Join(tempDir, "extracted")

	// Create test zip file
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Create zip: %v", err)
	}
	zw := zip.NewWriter(f)

	w1, err := zw.Create("server.js")
	if err != nil {
		t.Fatalf("zw.Create: %v", err)
	}
	_, _ = w1.Write([]byte("console.log('hello from test');"))

	w2, err := zw.Create("package.json")
	if err != nil {
		t.Fatalf("zw.Create: %v", err)
	}
	_, _ = w2.Write([]byte(`{"name":"test-app","version":"1.0.0"}`))

	if err := zw.Close(); err != nil {
		t.Fatalf("zw.Close: %v", err)
	}
	_ = f.Close()

	// Extract
	if err := ExtractArchive(zipPath, destDir); err != nil {
		t.Fatalf("ExtractArchive: %v", err)
	}

	// Verify extracted files
	serverJs := filepath.Join(destDir, "server.js")
	if data, err := os.ReadFile(serverJs); err != nil || string(data) != "console.log('hello from test');" {
		t.Errorf("expected server.js to be extracted properly, got data=%s, err=%v", string(data), err)
	}

	pkgJson := filepath.Join(destDir, "package.json")
	if data, err := os.ReadFile(pkgJson); err != nil || len(data) == 0 {
		t.Errorf("expected package.json to be extracted properly, got data=%s, err=%v", string(data), err)
	}
}

func TestExtractArchiveZipSlipProtection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "paas-zip-slip-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "malicious.zip")
	destDir := filepath.Join(tempDir, "extracted")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Create zip: %v", err)
	}
	zw := zip.NewWriter(f)

	// Try writing outside destDir via ../../evil.txt
	w, err := zw.Create("../../evil.txt")
	if err != nil {
		t.Fatalf("zw.Create: %v", err)
	}
	_, _ = w.Write([]byte("malicious content"))
	_ = zw.Close()
	_ = f.Close()

	err = ExtractArchive(zipPath, destDir)
	if err == nil {
		t.Errorf("expected error on zip slip attack, but got nil")
	}
}
