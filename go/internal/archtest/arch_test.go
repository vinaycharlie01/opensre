// Package archtest enforces the hexagon's dependency rules, the Go
// counterpart of the Python repo's import-linter contracts.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const module = "github.com/Tracer-Cloud/opensre/go/"

// rule: packages under dir may import first-party packages only under allow.
type rule struct {
	dir   string
	allow []string
	// thirdParty permits non-stdlib, non-first-party imports.
	thirdParty bool
}

var rules = []rule{
	// The domain is pure: only other domain packages and the stdlib.
	{dir: "internal/core/domain", allow: []string{"internal/core/domain/"}},
	// Ports describe boundaries in domain terms.
	{dir: "internal/core/ports", allow: []string{"internal/core/domain/"}},
	// Services orchestrate domain + ports; never adapters or platform.
	{dir: "internal/core/services", allow: []string{"internal/core/domain/", "internal/core/ports", "internal/core/services/"}},
	// Adapters depend inward on the core, never on each other's sides.
	{dir: "internal/adapters/secondary", allow: []string{"internal/core/domain/", "internal/core/ports"}, thirdParty: true},
	{dir: "internal/adapters/primary", allow: []string{"internal/core/domain/", "internal/core/ports"}},
	{dir: "internal/platform", allow: nil},
}

func repoRoot(t *testing.T) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestDependencyRules(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	for _, r := range rules {
		err := filepath.WalkDir(filepath.Join(root, r.dir), func(path string, d fs.DirEntry, err error) error {
			// Rules govern production code; tests may wire real adapters.
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				if !strings.HasPrefix(p, module) {
					if !r.thirdParty && strings.Contains(strings.SplitN(p, "/", 2)[0], ".") {
						t.Errorf("%s imports third-party %s; only secondary adapters may", rel, p)
					}
					continue
				}
				if !allowed(strings.TrimPrefix(p, module), r.allow) {
					t.Errorf("%s imports %s, outside its allowed layers %v", rel, p, r.allow)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func allowed(p string, allow []string) bool {
	for _, a := range allow {
		if p == strings.TrimSuffix(a, "/") || strings.HasPrefix(p, a) {
			return true
		}
	}
	return false
}
