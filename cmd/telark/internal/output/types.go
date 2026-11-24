package output

import "io"

type Format string

type Printer struct {
	w     io.Writer
	color bool
}
