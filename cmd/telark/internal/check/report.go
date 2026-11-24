package check

import (
	"fmt"
	"io"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/output"
)

const (
	statusPass Status = constants.StatusPass
	statusWarn Status = constants.StatusWarn
	statusFail Status = constants.StatusFail
)

func Pass(name, message string) Result {
	return Result{Name: name, Status: statusPass, Message: message}
}

func Warn(name, message string) Result {
	return Result{Name: name, Status: statusWarn, Message: message}
}

func Fail(name, message string) Result {
	return Result{Name: name, Status: statusFail, Message: message}
}

func ReadFailed(name, what string, err error) Result {
	return Fail(name, fmt.Sprintf(constants.MsgReadFailed, what, err))
}

func failed(result Result) bool {
	return result.Status == statusFail
}

func (r Report) Err() error {
	count := constants.DefaultInitValue
	for _, result := range r.Checks {
		if failed(result) {
			count++
		}
	}
	if count == constants.DefaultInitValue {
		return nil
	}
	return fmt.Errorf(constants.ErrChecksFailed, count, len(r.Checks))
}

func (r Report) Write(w io.Writer, format output.Format) error {
	if format != output.FormatTable {
		return output.Write(w, format, r)
	}
	r.Print(w)
	return nil
}

func (r Report) Print(w io.Writer) {
	printer := output.NewPrinter(w, output.UseColor(w))
	lines := map[Status]func(string){statusPass: printer.Pass, statusWarn: printer.Warn, statusFail: printer.Fail}
	for _, result := range r.Checks {
		lines[result.Status](result.Message)
	}
}
