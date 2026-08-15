// Package config loads redirect/rewrite rule definitions from a JSON file
// on disk, e.g.:
//
//	{
//	  "rules": [
//	    { "source": "/home", "destination": "/", "action": "redirect" },
//	    { "source": "/*", "destination": "/index.html", "action": "rewrite" }
//	  ]
//	}
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/example/render-redirects/internal/rules"
)

// File is the on-disk shape of the rules config.
type File struct {
	Rules []rules.Rule `json:"rules"`
}

// Load reads and parses the rules file at path. It does not compile the
// rules — call rules.NewSet on the result to validate and compile.
func Load(path string) ([]rules.Rule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return f.Rules, nil
}
