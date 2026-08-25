package jpeg2000

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const jpeg2000ModulePath = "github.com/cocosip/go-dicom-codecs/jpeg2000"

func TestJPEG2000ArchitectureHasNoFamilyViolations(t *testing.T) {
	t.Parallel()
	violations := collectJPEG2000ArchitectureViolations(t)
	if len(violations) != 0 {
		t.Fatalf("JPEG 2000 architecture contains family violations: %v", violations)
	}
}

func TestCommonCodestreamOwnsSharedParserAndModels(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"markers.go", "types.go", "parser.go"} {
		path := filepath.Join("internal", "common", "codestream", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("common codestream file %s: %v", path, err)
		}
	}
	if _, err := os.Stat("codestream"); !os.IsNotExist(err) {
		t.Errorf("legacy codestream package must be deleted, stat error = %v", err)
	}
}

func TestOpenJPEGConcreteEngineOwnsClassicImplementation(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"openjpeg/encoder.go",
		"openjpeg/decoder.go",
		"openjpeg/mqc",
		"openjpeg/t1",
		"openjpeg/t2",
		"openjpeg/colorspace",
		"openjpeg/wavelet",
	} {
		if _, err := os.Stat(filepath.FromSlash(path)); err != nil {
			t.Errorf("OpenJPEG-owned path %s: %v", path, err)
		}
	}
	for _, path := range []string{"mqc", "t1", "t2", "colorspace", "wavelet"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("legacy implementation path %s must be deleted, stat error = %v", path, err)
		}
	}
	if !nonTestPackageImports(t, ".", jpeg2000ModulePath+"/openjpeg") {
		t.Errorf("classic jpeg2000 facade does not import concrete OpenJPEG engine")
	}
}

func TestOpenJPEGEngineContainsNoHTPolicy(t *testing.T) {
	t.Parallel()
	violations := collectFamilyPolicyViolations(t, "openjpeg")
	if len(violations) != 0 {
		t.Fatalf("OpenJPEG engine contains HT policy: %v", violations)
	}
}

func TestOpenJPHOwnsCleanupDirectly(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"htj2k/openjph/cleanup/encoder.go",
		"htj2k/openjph/cleanup/decoder.go",
		"htj2k/openjph/cleanup/mel.go",
		"htj2k/openjph/cleanup/vlc.go",
		"htj2k/openjph/cleanup/uvlc.go",
		"htj2k/openjph/cleanup/magsgn.go",
	} {
		if _, err := os.Stat(filepath.FromSlash(path)); err != nil {
			t.Errorf("OpenJPH-owned cleanup path %s: %v", path, err)
		}
	}
	for _, path := range []string{
		"htj2k/mel.go",
		"htj2k/magsgn.go",
		"htj2k/uvlc_encoder.go",
		"htj2k/vlc_encoder.go",
		"htj2k/openjph/mqc",
		"htj2k/openjph/t1",
	} {
		if _, err := os.Stat(filepath.FromSlash(path)); !os.IsNotExist(err) {
			t.Errorf("legacy cleanup path %s must be deleted, stat error = %v", path, err)
		}
	}
	violations := collectIdentifierViolations(t, "htj2k/openjph", forbiddenOpenJPHTransitionIdentifier)
	if len(violations) != 0 {
		t.Fatalf("OpenJPH engine contains transition policy: %v", violations)
	}
}

func collectIdentifierViolations(t *testing.T, root string, forbidden func(string) bool) []string {
	t.Helper()
	fset := token.NewFileSet()
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && forbidden(ident.Name) {
				position := fset.Position(ident.Pos())
				violations = append(violations, architectureViolation("transition-identifier", filepath.ToSlash(path), position.Line, ident.Name))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inspect transition identifiers in %s: %v", root, err)
	}
	sort.Strings(violations)
	return violations
}

func forbiddenOpenJPHTransitionIdentifier(name string) bool {
	switch name {
	case "HTJ2KMode", "isHTJ2K", "SetHTJ2KMode", "BlockEncoderFactory", "BlockDecoderFactory", "blockDecoderFactory", "SetBlockDecoderFactory":
		return true
	default:
		return false
	}
}

func collectFamilyPolicyViolations(t *testing.T, root string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.Ident:
				if forbiddenOpenJPEGFamilyIdentifier(value.Name) {
					position := fset.Position(value.Pos())
					violations = append(violations, architectureViolation("family-identifier", filepath.ToSlash(path), position.Line, value.Name))
				}
			case *ast.BinaryExpr:
				if markerFamilyInference(value) {
					position := fset.Position(value.Pos())
					violations = append(violations, architectureViolation("marker-selector", filepath.ToSlash(path), position.Line, "CodeBlockStyle&0x40"))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inspect family policy in %s: %v", root, err)
	}
	sort.Strings(violations)
	return violations
}

func forbiddenOpenJPEGFamilyIdentifier(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "htj2k") ||
		strings.Contains(lower, "openjph") ||
		name == "BlockEncoderFactory" ||
		name == "BlockDecoderFactory"
}

func nonTestPackageImports(t *testing.T, dir, wanted string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package directory %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse imports from %s: %v", entry.Name(), parseErr)
		}
		for _, spec := range file.Imports {
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr == nil && path == wanted {
				return true
			}
		}
	}
	return false
}

func collectJPEG2000ArchitectureViolations(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	violations := make([]string, 0)
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, "."+string(filepath.Separator)))
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if reason := forbiddenJPEG2000Import(rel, importPath); reason != "" {
				position := fset.Position(spec.Pos())
				violations = append(violations, architectureViolation("import", rel, position.Line, importPath+" ("+reason+")"))
			}
		}
		if rootOrSharedJPEG2000File(rel) {
			fullFile, fullParseErr := parser.ParseFile(fset, path, nil, 0)
			if fullParseErr != nil {
				return fullParseErr
			}
			ast.Inspect(fullFile, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.Ident:
					if forbiddenFamilySelector(value.Name) {
						position := fset.Position(value.Pos())
						violations = append(violations, architectureViolation("selector", rel, position.Line, value.Name))
					}
				case *ast.BasicLit:
					if value.Kind == token.STRING {
						literal, unquoteErr := strconv.Unquote(value.Value)
						if unquoteErr == nil && strings.HasPrefix(literal, "1.2.840.10008.1.2.4.20") {
							position := fset.Position(value.Pos())
							violations = append(violations, architectureViolation("uid-selector", rel, position.Line, literal))
						}
					}
				case *ast.BinaryExpr:
					if markerFamilyInference(value) {
						position := fset.Position(value.Pos())
						violations = append(violations, architectureViolation("marker-selector", rel, position.Line, "CodeBlockStyle&0x40"))
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect JPEG 2000 architecture: %v", err)
	}
	sort.Strings(violations)
	return violations
}

func markerFamilyInference(expr *ast.BinaryExpr) bool {
	if expr.Op != token.EQL && expr.Op != token.NEQ {
		return false
	}
	hasCodeBlockStyle := false
	hasHTStyleBit := false
	ast.Inspect(expr, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			hasCodeBlockStyle = hasCodeBlockStyle || value.Name == "CodeBlockStyle"
		case *ast.BasicLit:
			hasHTStyleBit = hasHTStyleBit || value.Kind == token.INT && value.Value == "0x40"
		}
		return true
	})
	return hasCodeBlockStyle && hasHTStyleBit
}

func forbiddenJPEG2000Import(source, importPath string) string {
	switch {
	case strings.HasPrefix(source, "internal/common/") &&
		(importPath == jpeg2000ModulePath+"/openjpeg" ||
			strings.HasPrefix(importPath, jpeg2000ModulePath+"/openjpeg/") ||
			importPath == jpeg2000ModulePath+"/htj2k" ||
			strings.HasPrefix(importPath, jpeg2000ModulePath+"/htj2k/")):
		return "common must not depend on a codec family"
	case strings.HasPrefix(source, "openjpeg/") &&
		(importPath == jpeg2000ModulePath+"/htj2k" || strings.HasPrefix(importPath, jpeg2000ModulePath+"/htj2k/")):
		return "OpenJPEG must not depend on OpenJPH"
	case strings.HasPrefix(source, "htj2k/openjph/") &&
		(importPath == jpeg2000ModulePath+"/openjpeg" || strings.HasPrefix(importPath, jpeg2000ModulePath+"/openjpeg/")):
		return "OpenJPH must not depend on OpenJPEG"
	case strings.HasPrefix(source, "htj2k/") && importPath == jpeg2000ModulePath:
		return "HTJ2K adapter must use its concrete OpenJPH engine"
	default:
		return ""
	}
}

func rootOrSharedJPEG2000File(path string) bool {
	if !strings.Contains(path, "/") {
		return true
	}
	for _, prefix := range []string{"codestream/", "colorspace/", "mqc/", "t1/", "t2/", "wavelet/", "internal/common/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func forbiddenFamilySelector(name string) bool {
	switch name {
	case "HTJ2KMode", "isHTJ2K", "IsHTJ2K", "SetHTJ2KMode":
		return true
	default:
		return false
	}
}

func architectureViolation(kind, path string, line int, detail string) string {
	return kind + ":" + path + ":" + strconv.Itoa(line) + ":" + detail
}
