package output

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
)

const message = "the check"

func TestFormat(t *testing.T) {
	var format output.Format
	testutil.Equal(t, "type", format.Type(), "format")
	for _, value := range []string{"table", "json", "yaml"} {
		if err := format.Set(value); err != nil {
			t.Fatalf("set %q: %v", value, err)
		}
		testutil.Equal(t, "string", format.String(), value)
	}
	err := format.Set("xml")
	if err == nil {
		t.Fatal("set xml: want an error")
	}
	testutil.Equal(t, "error", err.Error(), "must be table, json or yaml")
	testutil.Equal(t, "kept", format, output.FormatYAML)
}

func TestWrite(t *testing.T) {
	cases := []struct {
		format output.Format
		want   string
	}{
		{output.FormatJSON, "{\n  \"key\": \"value\"\n}\n"},
		{output.FormatYAML, "key: value\n"},
	}
	for _, c := range cases {
		t.Run(string(c.format), func(t *testing.T) {
			var buffer bytes.Buffer
			if err := output.Write(&buffer, c.format, map[string]string{"key": "value"}); err != nil {
				t.Fatal(err)
			}
			testutil.Equal(t, "output", buffer.String(), c.want)
		})
	}
}

func TestWriteErrors(t *testing.T) {
	for _, format := range []output.Format{output.FormatJSON, output.FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			if err := output.Write(&bytes.Buffer{}, format, make(chan int)); err == nil {
				t.Error("unencodable value: want an error")
			}
			err := output.Write(testutil.FailingWriter{}, format, message)
			testutil.Equal(t, "write error", err, testutil.ErrWrite)
		})
	}
}

func TestPrinter(t *testing.T) {
	cases := []struct {
		name  string
		color bool
		print func(*output.Printer, string)
		want  string
	}{
		{"pass", false, (*output.Printer).Pass, "✔ the check\n"},
		{"warn", false, (*output.Printer).Warn, "‼ the check\n"},
		{"fail", false, (*output.Printer).Fail, "× the check\n"},
		{"pass in color", true, (*output.Printer).Pass, "\x1b[32m✔\x1b[0m the check\n"},
		{"warn in color", true, (*output.Printer).Warn, "\x1b[33m‼\x1b[0m the check\n"},
		{"fail in color", true, (*output.Printer).Fail, "\x1b[31m×\x1b[0m the check\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buffer bytes.Buffer
			c.print(output.NewPrinter(&buffer, c.color), message)
			testutil.Equal(t, "line", buffer.String(), c.want)
		})
	}
}

// A terminal is a character device, like os.DevNull, while files, pipes and buffers get no color.
// An empty NO_COLOR counts as unset, which also keeps the caller's own NO_COLOR out of the test.
func TestUseColor(t *testing.T) {
	device, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = device.Close() })
	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })

	t.Setenv("NO_COLOR", "")
	testutil.Equal(t, "buffer", output.UseColor(&bytes.Buffer{}), false)
	testutil.Equal(t, "file", output.UseColor(file), false)
	testutil.Equal(t, "character device", output.UseColor(device), true)
	t.Setenv("NO_COLOR", "1")
	testutil.Equal(t, "character device with NO_COLOR", output.UseColor(device), false)
}
