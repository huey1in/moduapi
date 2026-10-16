package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Protect the central promise: adding a plugin cannot make its implementation
// or a business runtime an import dependency of the kernel/SDK.
func TestLayerImportBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	module := "github.com/huey1in/moduapi/"
	for layer, forbidden := range map[string][]string{"core": {"runtime/", "plugins/", "internal/app"}, "sdk": {"core/", "runtime/", "plugins/", "internal/app"}, "runtime": {"core/", "plugins/", "internal/app"}, "plugins/official": {"core/", "runtime/", "internal/app"}} {
		e := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(layer)), func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, e := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if e != nil {
				return e
			}
			for _, im := range f.Imports {
				imported, e := strconv.Unquote(im.Path.Value)
				if e != nil {
					return e
				}
				for _, bad := range forbidden {
					if strings.HasPrefix(imported, module+bad) {
						t.Errorf("%s imports forbidden layer %s", path, imported)
					}
				}
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
	}
}
