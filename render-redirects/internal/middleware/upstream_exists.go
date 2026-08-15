package middleware

import (
	"net/http"
	"net/url"

	"github.com/example/render-redirects/internal/rules"
)

// UpstreamExister checks whether a resource exists by issuing a HEAD
// request to the nginx origin. Use this instead of FileSystemExister when
// the redirector process doesn't have direct filesystem access to the
// static content (e.g. it runs on a different host or in a different
// container without a shared volume).
//
// This costs a network round trip per request, so FileSystemExister is
// preferred when a shared volume is available.
type UpstreamExister struct {
	Client   *http.Client
	Upstream *url.URL
}

var _ rules.Exister = UpstreamExister{}

// Exists implements rules.Exister.
func (u UpstreamExister) Exists(reqPath string) bool {
	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}

	target := *u.Upstream
	target.Path = reqPath

	req, err := http.NewRequest(http.MethodHead, target.String(), nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
