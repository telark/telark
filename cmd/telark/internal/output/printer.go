package output

import (
	"fmt"
	"io"
	"os"

	"github.com/telark/telark/cmd/telark/internal/constants"
)

func NewPrinter(w io.Writer, color bool) *Printer {
	return &Printer{w: w, color: color}
}

// The character-device bit stands in for isatty, so the check needs no per-OS terminal package.
func UseColor(w io.Writer) bool {
	if os.Getenv(constants.EnvNoColor) != constants.EmptyString {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != constants.DefaultInitValue
}

func (p *Printer) Pass(message string) {
	p.line(constants.ColorGreen, constants.SymbolPass, message)
}

func (p *Printer) Warn(message string) {
	p.line(constants.ColorYellow, constants.SymbolWarn, message)
}

func (p *Printer) Fail(message string) {
	p.line(constants.ColorRed, constants.SymbolFail, message)
}

func (p *Printer) line(color, symbol, message string) {
	mark := symbol
	if p.color {
		mark = color + symbol + constants.ColorReset
	}
	_, _ = fmt.Fprintf(p.w, constants.LineFormat, mark, message)
}
