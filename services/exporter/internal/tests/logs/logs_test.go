package logs

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

const serviceRoot = "../../.."

// A %s next to a credential noun means the credential itself reaches the log
// line; %+v/%#v dump whole structs and carry secrets along with them.
var forbidden = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"credential interpolated into message", regexp.MustCompile(`(?i)(token|credentialid|credential|password|secret|apikey)[^"]*%s`)},
	{"struct dump verb", regexp.MustCompile(`%[+#]v`)},
}

func TestNoCredentialsOrStructDumpsInMessages(t *testing.T) {
	fset := token.NewFileSet()

	err := filepath.WalkDir(serviceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !isScannableSource(path) {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil {
				return true
			}
			for _, rule := range forbidden {
				if rule.pattern.MatchString(value) {
					t.Errorf("%s: %s in %q", fset.Position(lit.Pos()), rule.name, value)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
}

func isScannableSource(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
