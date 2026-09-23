package logging

import (
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/telark/notifier/internal/constants"
)

const serviceRoot = "../../.."

var forbidden = []struct {
	name string
	re   *regexp.Regexp
}{
	{"whole message object as format argument", regexp.MustCompile(`fmt\.(Sprintf|Errorf)\([^)]*\)[^)]*,\s*(m|msg|msgs|message)\s*,`)},
	{"verbose struct formatting verb", regexp.MustCompile(`%[+#]v`)},
}

func TestNoWholeMessageLogging(t *testing.T) {
	var findings []string

	root, err := os.OpenRoot(serviceRoot)
	if err != nil {
		t.Fatalf("open service root: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	fsys := root.FS()

	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "tests" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(content), "\n") {
			for _, f := range forbidden {
				if f.re.MatchString(line) {
					findings = append(findings, path+":"+strconv.Itoa(i+1)+": "+f.name+": "+strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk service source: %v", err)
	}

	if len(findings) > constants.DefaultInitValue {
		t.Fatalf("log calls must never format a whole NATS message or struct:\n%s", strings.Join(findings, "\n"))
	}
}
