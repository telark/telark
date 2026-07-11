package shared

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/telark/discovery/internal/constants"
)

var stringBuilderPool = sync.Pool{
	New: func() any {
		sb := &strings.Builder{}
		sb.Grow(constants.StringBuilderSize)
		return sb
	},
}

var bufferPool = sync.Pool{
	New: func() any {
		buf := &bytes.Buffer{}
		buf.Grow(constants.BufferSize)
		return buf
	},
}

func GetStringBuilder() *strings.Builder {
	if sb, ok := stringBuilderPool.Get().(*strings.Builder); ok {
		return sb
	}
	return &strings.Builder{}
}

func PutStringBuilder(sb *strings.Builder) {
	sb.Reset()
	stringBuilderPool.Put(sb)
}

func GetBuffer() *bytes.Buffer {
	if buf, ok := bufferPool.Get().(*bytes.Buffer); ok {
		return buf
	}
	return &bytes.Buffer{}
}

func PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

func FormatKey(resourceType, namespace, name string) string {
	if name == constants.EmptyString {
		return ConcatWithPath(resourceType, namespace)
	}
	return ConcatWithPath(resourceType, namespace, name)
}

func ConcatWithDash[T any](values ...T) string {
	if len(values) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	if len(values) == constants.DefaultAddValue {
		return fmt.Sprint(values[0])
	}

	parts := make([]string, constants.DefaultInitValue, len(values))
	for _, v := range values {
		parts = append(parts, fmt.Sprint(v))
	}
	return concatWithSeparator(constants.DashSeparator, parts...)
}

func ConcatWithColon(values ...string) string {
	return concatWithSeparator(constants.ColonSeparator, values...)
}

func ConcatWithPath(values ...string) string {
	return concatWithSeparator(constants.PathSeparator, values...)
}

func concatWithSeparator(separator string, values ...string) string {
	if len(values) == constants.DefaultInitValue {
		return constants.EmptyString
	}

	if len(values) == constants.DefaultAddValue {
		return values[constants.DefaultInitValue]
	}

	if len(values) <= constants.SimpleConcatLimit {
		return strings.Join(values, separator)
	}

	// Use string builder for complex concatenations
	estimatedSize := len(values[0])
	for i := constants.DefaultAddValue; i < len(values); i++ {
		estimatedSize += len(separator) + len(values[i])
	}

	sb := GetStringBuilder()
	defer PutStringBuilder(sb)
	sb.Grow(estimatedSize)

	for i, value := range values {
		if i > constants.DefaultInitValue {
			sb.WriteString(separator)
		}
		sb.WriteString(value)
	}
	return sb.String()
}
