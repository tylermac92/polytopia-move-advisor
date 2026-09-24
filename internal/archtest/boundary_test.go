package archtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/tylermac92/polytopia-move-advisor"

// advisorPkgs are the packages that make up the advisor. They must never
// reach code that holds or serves the sandbox's true GameState.
var advisorPkgs = []string{"internal/search", "internal/eval", "internal/explain"}

// forbiddenForAdvisor may not appear anywhere in an advisor package's
// transitive dependencies.
var forbiddenForAdvisor = []string{"internal/sandbox", "internal/view", "internal/api"}

type goPackage struct {
	ImportPath string
	Imports    []string
	Deps       []string
}

// listPackages runs `go list -deps -json` on every package in the module and
// returns the module's own packages keyed by import path.
func listPackages(t *testing.T) map[string]goPackage {
	t.Helper()
	touchSources(t)
	out, err := exec.Command("go", "list", "-deps", "-json", module+"/...").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("go list: %v\n%s", err, ee.Stderr)
		}
		t.Fatalf("go list: %v", err)
	}
	pkgs := map[string]goPackage{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		if strings.HasPrefix(p.ImportPath, module+"/") {
			pkgs[p.ImportPath] = p
		}
	}
	return pkgs
}

// touchSources reads every Go file in the module. go test caches results
// keyed on the files a test opens, and it cannot see the files `go list` reads
// in a subprocess; reading them here makes any import change rerun this test.
func touchSources(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") || d.Name() == "go.mod" {
			_, err = os.ReadFile(path)
		}
		return err
	})
	if err != nil {
		t.Fatalf("read module sources: %v", err)
	}
}

func pkg(t *testing.T, pkgs map[string]goPackage, rel string) goPackage {
	t.Helper()
	p, ok := pkgs[module+"/"+rel]
	if !ok {
		t.Fatalf("package %s not found; the boundary test would pass vacuously", rel)
	}
	return p
}

// TestAdvisorImportBoundary is the secondary enforcement of the information
// boundary (see "Information boundary" in the technical design). It proves
// where a true GameState cannot come from; the mutation test covers the rest.
func TestAdvisorImportBoundary(t *testing.T) {
	pkgs := listPackages(t)
	for _, rel := range forbiddenForAdvisor {
		pkg(t, pkgs, rel)
	}

	for _, rel := range advisorPkgs {
		p := pkg(t, pkgs, rel)
		for _, bad := range forbiddenForAdvisor {
			if slices.Contains(p.Deps, module+"/"+bad) {
				t.Errorf("%s has an import path to %s", rel, bad)
			}
		}
	}

	api := pkg(t, pkgs, "internal/api")
	for _, rel := range advisorPkgs {
		if rel == "internal/search" {
			continue
		}
		if slices.Contains(api.Imports, module+"/"+rel) {
			t.Errorf("internal/api imports %s; it may reach the advisor only through internal/search", rel)
		}
	}
}
