package endpoints_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	endpointsRoot   = "../../endpoints"
	goSuffix        = ".go"
	endpointType    = "Endpoint"
	minEndpoints    = 50
	unquoteFailed   = "%s: cannot unquote %s: %v"
	notLiteral      = "%s is not a string literal; the scan cannot check it"
	verbSegmentHint = "%s = %q ends in a verb segment; the HTTP method carries the action"
	finderHint      = "%s = %q contains a findby segment; address the resource or use internal/*"
	tooFewHint      = "found %d endpoint constants under %s, want at least %d: the scan is broken"
)

var verbSuffix = regexp.MustCompile(`/(get|create|patch|delete)$`)

func endpointConstants(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(endpointsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, goSuffix) {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		collectEndpoints(t, file, found)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", endpointsRoot, err)
	}
	return found
}

func collectEndpoints(t *testing.T, file *ast.File, found map[string]string) {
	t.Helper()
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok {
				collectSpec(t, file.Name.Name, value, found)
			}
		}
	}
}

func collectSpec(t *testing.T, pkg string, value *ast.ValueSpec, found map[string]string) {
	t.Helper()
	selector, ok := value.Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != endpointType {
		return
	}
	for i, name := range value.Names {
		literal, ok := value.Values[i].(*ast.BasicLit)
		if !ok {
			t.Fatalf(notLiteral, name.Name)
		}
		path, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatalf(unquoteFailed, name.Name, literal.Value, err)
		}
		found[pkg+"."+name.Name] = path
	}
}

func TestEndpointPathsAreResourceOriented(t *testing.T) {
	found := endpointConstants(t)
	if len(found) < minEndpoints {
		t.Fatalf(tooFewHint, len(found), endpointsRoot, minEndpoints)
	}
	for name, path := range found {
		if verbSuffix.MatchString(path) {
			t.Errorf(verbSegmentHint, name, path)
		}
		if strings.Contains(strings.ToLower(path), "findby") {
			t.Errorf(finderHint, name, path)
		}
	}
}
