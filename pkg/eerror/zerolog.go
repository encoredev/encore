package eerror

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type errorMeta struct {
	Frames []uintptr      `json:"frames"`
	Meta   map[string]any `json:"meta"`
}

// ZeroLogStackMarshaller will encode the following in the Zerolog error stack field:
// - the deepest error stack trace
// - the error meta data collected from all errors
//
// This can then be extracted by ZeroLogConsoleExtraFormatter
func ZeroLogStackMarshaller(err error) interface{} {
	var frames []uintptr
	for _, frame := range BottomStackTraceFrom(err) {
		// Account for the stack trace being 1 frame deeper than the error
		frames = append(frames, uintptr(frame-1))
	}
	frames = filterFrames(frames)

	return &errorMeta{
		Frames: frames,
		Meta:   MetaFrom(err),
	}
}

// ZeroLogConsoleExtraFormatter extracts the extra fields from the error and formats them for console output
//
// This field can be passed to a zerolog.ConsoleWriter as it's ExtraFieldFormatter
func ZeroLogConsoleExtraFormatter(event map[string]any, buf *bytes.Buffer) error {
	stackFieldValue, found := event[zerolog.ErrorStackFieldName]
	if !found || stackFieldValue == nil {
		return nil
	}

	// Marshal the stack field to our error meta
	var errorMeta errorMeta
	jsonBytes, err := json.Marshal(stackFieldValue)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonBytes, &errorMeta); err != nil {
		return err
	}

	// Log any additional meta data
	if len(errorMeta.Meta) > 0 {
		// Get the fields not already set in the log, and then sort them
		fields := make([]string, 0, len(errorMeta.Meta))
		for field := range errorMeta.Meta {
			if _, alreadySet := event[field]; !alreadySet {
				fields = append(fields, field)
			}
		}
		sort.Strings(fields)

		// Then log the additional fields
		for _, field := range fields {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}

			fieldName := fmt.Sprintf("\x1b[%dm%v\x1b[0m=", 36, field)
			buf.WriteString(fieldName)

			switch fValue := errorMeta.Meta[field].(type) {
			case string:
				if needsQuote(fValue) {
					buf.WriteString(strconv.Quote(fValue))
				} else {
					buf.WriteString(fValue)
				}
			case json.Number:
				buf.WriteString(fValue.String())
			default:
				b, err := zerolog.InterfaceMarshalFunc(fValue)
				if err != nil {
					buf.WriteString(fmt.Sprintf("[error: \x1b[%dm%v\x1b[0m=]", 31, err))
				} else {
					buf.WriteString(string(b))
				}
			}
		}
	}

	// Now print a stack trace if we have it
	if len(errorMeta.Frames) > 0 {
		// Find the longest function name so we can align them
		longestFunc := 0
		for _, frame := range errorMeta.Frames {
			module, function := frameToModuleFunc(frame)
			fName := fmt.Sprintf("%s.%s", module, function)
			if len(fName) > longestFunc {
				longestFunc = len(fName)
			}
		}

		// Print the stack trace
		for i, frame := range errorMeta.Frames {
			module, function := frameToModuleFunc(frame)
			filename, line := frameToFileLine(frame)

			buf.WriteString(fmt.Sprintf(
				"\n\tat %s.%s%s %s:%d",
				fmt.Sprintf("\x1b[%dm%v\x1b[0m", 90, module),
				fmt.Sprintf("\x1b[%dm%v\x1b[0m", 35, function),
				strings.Repeat(" ", longestFunc-(len(function)+len(module)+1)),
				filename, line,
			))

			if i > 5 {
				buf.WriteString(fmt.Sprintf("\n\t... remaining %d frames omitted ...", len(errorMeta.Frames)-(i+1)))
				break
			}
		}
	}

	return nil
}

func frameToModuleFunc(frame uintptr) (module string, name string) {
	fn := runtime.FuncForPC(frame)
	if fn == nil {
		return "", "unknown"
	}

	name = fn.Name()
	if idx := strings.LastIndex(name, "."); idx != -1 {
		module = name[:idx]
		name = name[idx+1:]
	}

	module = strings.TrimPrefix(module, "encr.dev/")
	name = strings.Replace(name, "·", ".", -1)
	return
}

func frameToFileLine(frame uintptr) (file string, line int) {
	fn := runtime.FuncForPC(frame)
	if fn == nil {
		return "unknown", 0
	}

	filename, line := fn.FileLine(frame)
	filename = strings.TrimPrefix(filename, projectSourcePath)

	return filename, line
}

// filterFrames filters out stack frames from zerolog and this package.
func filterFrames(frames []uintptr) []uintptr {
	if len(frames) == 0 {
		return nil
	}
	filteredFrames := make([]uintptr, 0, len(frames))

	for _, frame := range frames {
		module, _ := frameToModuleFunc(frame)
		if strings.HasPrefix(module, "github.com/rs/zerolog") {
			continue
		} else if strings.HasPrefix(module, "github.com/spf13/cobra") {
			continue
		}
		filteredFrames = append(filteredFrames, frame)
	}
	return filteredFrames
}

// needsQuote returns true when the string s should be quoted in output.
func needsQuote(s string) bool {
	for i := range s {
		if s[i] < 0x20 || s[i] > 0x7e || s[i] == ' ' || s[i] == '\\' || s[i] == '"' {
			return true
		}
	}
	return false
}

// LogWithMeta merges in the metadata from the errors into the log context
func LogWithMeta(evt *zerolog.Event, err error) *zerolog.Event {
	if err == nil {
		return evt
	}

	meta := MetaFrom(err)
	for key, value := range meta {
		switch value := value.(type) {
		case json.RawMessage:
			evt = evt.RawJSON(key, value)
		case error:
			evt = evt.AnErr(key, value)
		case time.Time:
			evt = evt.Time(key, value)
		case time.Duration:
			evt = evt.Dur(key, value)
		case net.IP:
			evt = evt.IPAddr(key, value)
		case net.IPNet:
			evt = evt.IPPrefix(key, value)
		case net.HardwareAddr:
			evt = evt.MACAddr(key, value)
		case string:
			evt = evt.Str(key, value)
		case int:
			evt = evt.Int(key, value)
		case int8:
			evt = evt.Int8(key, value)
		case int16:
			evt = evt.Int16(key, value)
		case int32:
			evt = evt.Int32(key, value)
		case int64:
			evt = evt.Int64(key, value)
		case uint:
			evt = evt.Uint(key, value)
		case uint8:
			evt = evt.Uint8(key, value)
		case uint16:
			evt = evt.Uint16(key, value)
		case uint32:
			evt = evt.Uint32(key, value)
		case uint64:
			evt = evt.Uint64(key, value)
		case float32:
			evt = evt.Float32(key, value)
		case float64:
			evt = evt.Float64(key, value)
		case bool:
			evt = evt.Bool(key, value)
		case []error:
			evt = evt.Errs(key, value)
		case []time.Time:
			evt = evt.Times(key, value)
		case []time.Duration:
			evt = evt.Durs(key, value)
		case []string:
			evt = evt.Strs(key, value)
		case []int:
			evt = evt.Ints(key, value)
		case []int8:
			evt = evt.Ints8(key, value)
		case []int16:
			evt = evt.Ints16(key, value)
		case []int32:
			evt = evt.Ints32(key, value)
		case []int64:
			evt = evt.Ints64(key, value)
		case []uint:
			evt = evt.Uints(key, value)
		case []byte: // uint8 / byte are the same thing so we'll default to bytes
			evt = evt.Bytes(key, value)
		case []uint16:
			evt = evt.Uints16(key, value)
		case []uint32:
			evt = evt.Uints32(key, value)
		case []uint64:
			evt = evt.Uints64(key, value)
		case []float32:
			evt = evt.Floats32(key, value)
		case []float64:
			evt = evt.Floats64(key, value)
		case []bool:
			evt = evt.Bools(key, value)
		default:
			evt = evt.Interface(key, value)
		}
	}
	return evt
}
