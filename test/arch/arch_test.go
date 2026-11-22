package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readModulePath extracts the module path from the root go.mod file so that
// this test continues to work after `kratos new` rewrites the module name.
func readModulePath(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	t.Fatal("module directive not found in go.mod")
	return ""
}

// TestArchitectureBoundaries enforces the template's hard package and module
// boundary rules across the entire repository:
//
//  1. pkg/...      -> cannot import core, ent, internal, api
//  2. ent/...      -> cannot import core, internal, api
//  3. core/...     -> cannot import internal, api
//  4. internal/    -> only contains business module subdirectories (no loose Go files),
//     and each module only uses the standard sub-packages (server, service, biz, model)
//  5. Cross-module -> internal/<a>/... cannot import internal/<b>/...
//  6. Intra-module ->
//     - internal/<m>/biz     cannot import ent or internal/<m>/{model,service,server}
//     - internal/<m>/model   cannot import internal/<m>/{service,server} or api
//     - internal/<m>/service cannot import ent or internal/<m>/{model,server}
//     - internal/<m>/server  cannot import ent or internal/<m>/{model,biz}
func TestArchitectureBoundaries(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	mod := readModulePath(t, root)

	// Verify internal/ only contains module directories and each module only
	// contains allowed sub-packages.
	internalDir := filepath.Join(root, "internal")
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		t.Fatalf("read internal dir: %v", err)
	}
	allowedSubDirs := map[string]bool{
		"server":  true,
		"service": true,
		"biz":     true,
		"model":   true,
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			if strings.HasSuffix(entry.Name(), ".go") {
				t.Errorf("internal/%s: loose Go files are not allowed directly in internal/; place all code inside a module directory internal/<module>/", entry.Name())
			}
			continue
		}
		modName := entry.Name()
		modEntries, err := os.ReadDir(filepath.Join(internalDir, modName))
		if err != nil {
			t.Fatalf("read internal/%s: %v", modName, err)
		}
		for _, sub := range modEntries {
			if sub.IsDir() && !allowedSubDirs[sub.Name()] {
				t.Errorf("internal/%s/%s: unexpected module sub-package %q (allowed: server, service, biz, model)", modName, sub.Name(), sub.Name())
			}
		}
	}

	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".idea" || name == ".vscode" || name == "vendor" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		pkgDir := filepath.ToSlash(filepath.Dir(rel))

		fileAST, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse %s: %v", rel, err)
			return nil
		}

		for _, imp := range fileAST.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(impPath, mod+"/") && impPath != mod {
				continue
			}
			target := strings.TrimPrefix(impPath, mod+"/")
			checkImportRule(t, rel, pkgDir, target, fset, imp)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

func checkImportRule(t *testing.T, fileRel, srcPkg, target string, fset *token.FileSet, imp *ast.ImportSpec) {
	t.Helper()
	pos := fset.Position(imp.Pos())

	hasPrefix := func(s, prefix string) bool {
		return s == prefix || strings.HasPrefix(s, prefix+"/")
	}

	switch {
	case hasPrefix(srcPkg, "pkg"):
		for _, forbidden := range []string{"core", "ent", "internal", "api", "cmd"} {
			if hasPrefix(target, forbidden) {
				t.Errorf("%s:%d: pkg (%s) must not import %s (%s)", fileRel, pos.Line, srcPkg, forbidden, target)
			}
		}

	case hasPrefix(srcPkg, "ent"):
		for _, forbidden := range []string{"core", "internal", "api", "cmd"} {
			if hasPrefix(target, forbidden) {
				t.Errorf("%s:%d: ent (%s) must not import %s (%s)", fileRel, pos.Line, srcPkg, forbidden, target)
			}
		}

	case hasPrefix(srcPkg, "core"):
		for _, forbidden := range []string{"internal", "api", "cmd"} {
			if hasPrefix(target, forbidden) {
				t.Errorf("%s:%d: core (%s) must not import %s (%s)", fileRel, pos.Line, srcPkg, forbidden, target)
			}
		}

	case hasPrefix(srcPkg, "internal"):
		srcParts := strings.Split(srcPkg, "/")
		if len(srcParts) < 2 {
			return
		}
		srcModule := srcParts[1]
		srcLayer := ""
		if len(srcParts) >= 3 {
			srcLayer = srcParts[2]
		}

		if hasPrefix(target, "cmd") {
			t.Errorf("%s:%d: internal module (%s) must not import cmd (%s)", fileRel, pos.Line, srcPkg, target)
			return
		}

		// Check cross-module imports: internal/<a>/... cannot import internal/<b>/...
		if hasPrefix(target, "internal") {
			targetParts := strings.Split(target, "/")
			if len(targetParts) >= 2 {
				targetModule := targetParts[1]
				if targetModule != srcModule {
					t.Errorf("%s:%d: module %q (%s) must not directly import another module %q (%s)",
						fileRel, pos.Line, srcModule, srcPkg, targetModule, target)
					return
				}
			}
		}

		// Intra-module layer rules (only root internal/<m> package may wire all 4 layers)
		switch srcLayer {
		case "biz":
			if hasPrefix(target, "ent") {
				t.Errorf("%s:%d: biz layer (%s) must not import ent (%s); access data via repo interface implemented in model",
					fileRel, pos.Line, srcPkg, target)
			}
			for _, forbiddenLayer := range []string{"model", "service", "server"} {
				if hasPrefix(target, "internal/"+srcModule+"/"+forbiddenLayer) {
					t.Errorf("%s:%d: biz layer (%s) must not import %s layer (%s)",
						fileRel, pos.Line, srcPkg, forbiddenLayer, target)
				}
			}

		case "model":
			if hasPrefix(target, "api") {
				t.Errorf("%s:%d: model layer (%s) must not import api DTOs (%s); speak domain objects from biz",
					fileRel, pos.Line, srcPkg, target)
			}
			for _, forbiddenLayer := range []string{"service", "server"} {
				if hasPrefix(target, "internal/"+srcModule+"/"+forbiddenLayer) {
					t.Errorf("%s:%d: model layer (%s) must not import %s layer (%s)",
						fileRel, pos.Line, srcPkg, forbiddenLayer, target)
				}
			}

		case "service":
			if hasPrefix(target, "ent") {
				t.Errorf("%s:%d: service layer (%s) must not import ent (%s)",
					fileRel, pos.Line, srcPkg, target)
			}
			for _, forbiddenLayer := range []string{"model", "server"} {
				if hasPrefix(target, "internal/"+srcModule+"/"+forbiddenLayer) {
					t.Errorf("%s:%d: service layer (%s) must not import %s layer (%s)",
						fileRel, pos.Line, srcPkg, forbiddenLayer, target)
				}
			}

		case "server":
			if hasPrefix(target, "ent") {
				t.Errorf("%s:%d: server layer (%s) must not import ent (%s)",
					fileRel, pos.Line, srcPkg, target)
			}
			for _, forbiddenLayer := range []string{"model", "biz"} {
				if hasPrefix(target, "internal/"+srcModule+"/"+forbiddenLayer) {
					t.Errorf("%s:%d: server layer (%s) must not import %s layer (%s)",
						fileRel, pos.Line, srcPkg, forbiddenLayer, target)
				}
			}
		}
	}
}
