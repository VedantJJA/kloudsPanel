package middleware

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/example/render-redirects/internal/rules"
)

// FileSystemExister checks a local directory tree for a matching static
// asset. It's meant to point at the same volume nginx serves from, so the
// "does a resource exist" check is exact and doesn't cost a network hop.
//
// A directory request (e.g. "/blog/") is treated as satisfied if it
// contains an index.html, matching how nginx (and most static hosts)
// resolve directory URLs.
type FileSystemExister struct {
	Root string
}

var _ rules.Exister = FileSystemExister{}

// Exists implements rules.Exister.
func (f FileSystemExister) Exists(reqPath string) bool {
	root := filepath.Clean(f.Root)
	full := filepath.Join(root, filepath.Clean("/"+reqPath))

	// Guard against escaping Root via "../" segments.
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return false
	}

	info, err := os.Stat(full)
	if err != nil {
		return false
	}
	if info.IsDir() {
		_, err := os.Stat(filepath.Join(full, "index.html"))
		return err == nil
	}
	return true
}
