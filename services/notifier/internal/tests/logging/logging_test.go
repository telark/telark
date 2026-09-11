package logging

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
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

	err := filepath.WalkDir(serviceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "tests" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(content), "\n") {
			for _, f := range forbidden {
				if f.re.MatchString(line) {
					findings = append(findings, filepath.ToSlash(path)+":"+strconv.Itoa(i+1)+": "+f.name+": "+strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk service source: %v", err)
	}

	if len(findings) > 0 {
		t.Fatalf("log calls must never format a whole NATS message or struct:\n%s", strings.Join(findings, "\n"))
	}
}
