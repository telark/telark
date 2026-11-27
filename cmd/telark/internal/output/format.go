package output

import (
	"encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"sigs.k8s.io/yaml"
)

const (
	FormatTable Format = constants.FormatTable
	FormatJSON  Format = constants.FormatJSON
	FormatYAML  Format = constants.FormatYAML
)

var (
	formats          = []string{constants.FormatTable, constants.FormatJSON, constants.FormatYAML}
	errUnknownFormat = errors.New(constants.ErrUnknownFormat)
)

func AddFormatFlag(cmd *cobra.Command, format *Format) {
	*format = FormatTable
	cmd.Flags().VarP(format, constants.FlagOutput, constants.FlagOutputShort, constants.FlagOutputUsage)
	_ = cmd.RegisterFlagCompletionFunc(constants.FlagOutput, cobra.FixedCompletions(formats, cobra.ShellCompDirectiveNoFileComp))
}

func (f *Format) String() string {
	return string(*f)
}

func (f *Format) Set(value string) error {
	if !slices.Contains(formats, value) {
		return errUnknownFormat
	}
	*f = Format(value)
	return nil
}

func (*Format) Type() string {
	return constants.FlagOutputType
}

func Write(w io.Writer, format Format, v any) error {
	data, err := marshal(format, v)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func marshal(format Format, v any) ([]byte, error) {
	if format == FormatYAML {
		return yaml.Marshal(v)
	}
	data, err := json.MarshalIndent(v, constants.EmptyString, constants.JSONIndent)
	if err != nil {
		return nil, err
	}
	return append(data, constants.NewLine...), nil
}
