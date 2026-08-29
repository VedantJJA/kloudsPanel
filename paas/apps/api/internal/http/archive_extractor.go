package http

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractArchive extracts a .zip, .tar.gz, .tgz, or .tar file to destDir safely.
func ExtractArchive(srcFile, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	lower := strings.ToLower(srcFile)
	if strings.HasSuffix(lower, ".zip") {
		return extractZip(srcFile, destDir)
	} else if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGz(srcFile, destDir)
	} else if strings.HasSuffix(lower, ".tar") {
		return extractTar(srcFile, destDir)
	}

	// Try zip first, then tar.gz as fallback
	if err := extractZip(srcFile, destDir); err == nil {
		return nil
	}
	return extractTarGz(srcFile, destDir)
}

func extractZip(srcFile, destDir string) error {
	r, err := zip.OpenReader(srcFile)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	destClean := filepath.Clean(destDir)

	// Check if all entries share a single root folder prefix (e.g. "my-project-master/")
	commonPrefix := ""
	if len(r.File) > 0 {
		firstSlash := strings.Index(r.File[0].Name, "/")
		if firstSlash > 0 {
			candidate := r.File[0].Name[:firstSlash+1]
			allMatch := true
			for _, f := range r.File {
				if !strings.HasPrefix(f.Name, candidate) {
					allMatch = false
					break
				}
			}
			if allMatch {
				commonPrefix = candidate
			}
		}
	}

	for _, f := range r.File {
		relName := f.Name
		if commonPrefix != "" && strings.HasPrefix(relName, commonPrefix) {
			relName = strings.TrimPrefix(relName, commonPrefix)
		}
		if relName == "" || relName == "." {
			continue
		}

		target := filepath.Join(destClean, filepath.FromSlash(relName))
		// Zip slip check
		if !strings.HasPrefix(filepath.Clean(target), destClean) {
			return fmt.Errorf("illegal file path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(srcFile, destDir string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return extractTarReader(tar.NewReader(gzr), destDir)
}

func extractTar(srcFile, destDir string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return extractTarReader(tar.NewReader(f), destDir)
}

func extractTarReader(tr *tar.Reader, destDir string) error {
	destClean := filepath.Clean(destDir)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		relName := filepath.FromSlash(header.Name)
		target := filepath.Join(destClean, relName)
		if !strings.HasPrefix(filepath.Clean(target), destClean) {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}
