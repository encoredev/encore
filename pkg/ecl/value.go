package ecl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ValueKind identifies the type of a Value.
type ValueKind int

const (
	NumberKind ValueKind = iota
	BoolKind
	StringKind
	SizeKind
	DurationKind
)

func (k ValueKind) String() string {
	switch k {
	case NumberKind:
		return "number"
	case BoolKind:
		return "bool"
	case StringKind:
		return "string"
	case SizeKind:
		return "size"
	case DurationKind:
		return "duration"
	default:
		return fmt.Sprintf("ValueKind(%d)", int(k))
	}
}

// Value is an ECL value: a number, bool, string, size, or duration.
// Sizes are stored canonically in bytes, durations in milliseconds.
type Value struct {
	Kind ValueKind
	Num  float64 // NumberKind: the number; SizeKind: bytes; DurationKind: milliseconds
	Str  string  // StringKind
	Bool bool    // BoolKind

	unit string // display unit for sizes/durations ("" picks one automatically)
}

// Number returns a numeric Value.
func Number(v float64) Value { return Value{Kind: NumberKind, Num: v} }

// Bool returns a boolean Value.
func Bool(b bool) Value { return Value{Kind: BoolKind, Bool: b} }

// String returns a string Value.
func String(s string) Value { return Value{Kind: StringKind, Str: s} }

// Size returns a size Value of v units, e.g. Size(512, "Mi").
func Size(v float64, unit string) (Value, error) {
	factor, ok := sizeUnits[unit]
	if !ok {
		return Value{}, fmt.Errorf("unknown size unit %q (valid units: %s)", unit, unitList(sizeUnits))
	}
	return Value{Kind: SizeKind, Num: v * factor, unit: unit}, nil
}

// Duration returns a duration Value of v units, e.g. Duration(30, "d").
func Duration(v float64, unit string) (Value, error) {
	factor, ok := durationUnits[unit]
	if !ok {
		return Value{}, fmt.Errorf("unknown duration unit %q (valid units: %s)", unit, unitList(durationUnits))
	}
	return Value{Kind: DurationKind, Num: v * factor, unit: unit}, nil
}

// MustParseQuantity parses a number with an optional unit suffix, such as
// "512Mi", "30d", or "2.5". It panics if the input is invalid; it is
// intended for tests and static initialization.
func MustParseQuantity(s string) Value {
	v, err := ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseQuantity parses a number with an optional unit suffix, such as
// "512Mi", "30d", or "2.5".
func ParseQuantity(s string) (Value, error) {
	i := 0
	for i < len(s) && (isDigit(s[i]) || s[i] == '.' || s[i] == '-') {
		i++
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return Value{}, fmt.Errorf("invalid quantity %q", s)
	}
	unit := s[i:]
	switch {
	case unit == "":
		return Number(num), nil
	case sizeUnits[unit] != 0:
		return Size(num, unit)
	case durationUnits[unit] != 0:
		return Duration(num, unit)
	default:
		return Value{}, fmt.Errorf("unknown unit %q in quantity %q", unit, s)
	}
}

var sizeUnits = map[string]float64{
	"B":  1,
	"KB": 1e3,
	"MB": 1e6,
	"GB": 1e9,
	"TB": 1e12,
	"Ki": 1 << 10,
	"Mi": 1 << 20,
	"Gi": 1 << 30,
	"Ti": 1 << 40,
}

var durationUnits = map[string]float64{
	"ms": 1,
	"s":  1000,
	"m":  60 * 1000,
	"h":  60 * 60 * 1000,
	"d":  24 * 60 * 60 * 1000,
}

func unitList(units map[string]float64) string {
	names := make([]string, 0, len(units))
	for u := range units {
		names = append(names, u)
	}
	sort.Slice(names, func(i, j int) bool {
		if units[names[i]] != units[names[j]] {
			return units[names[i]] < units[names[j]]
		}
		return names[i] < names[j]
	})
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// String renders the value as ECL source text.
func (v Value) String() string {
	switch v.Kind {
	case NumberKind:
		return formatFloat(v.Num)
	case BoolKind:
		return strconv.FormatBool(v.Bool)
	case StringKind:
		return strconv.Quote(v.Str)
	case SizeKind:
		return formatQuantity(v.Num, v.unit, sizeUnits, "B")
	case DurationKind:
		return formatQuantity(v.Num, v.unit, durationUnits, "ms")
	default:
		return fmt.Sprintf("<invalid value kind %d>", int(v.Kind))
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func formatQuantity(canonical float64, unit string, units map[string]float64, base string) string {
	if unit == "" {
		// Pick the largest unit that divides the value evenly.
		best := base
		bestFactor := units[base]
		for u, factor := range units {
			scaled := canonical / factor
			if scaled >= 1 && scaled == float64(int64(scaled)) && factor > bestFactor {
				best, bestFactor = u, factor
			}
		}
		unit = best
	}
	return formatFloat(canonical/units[unit]) + unit
}

// valuesEqual reports whether two values are equal. Values of different
// kinds are never equal.
func valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case NumberKind, SizeKind, DurationKind:
		return a.Num == b.Num
	case BoolKind:
		return a.Bool == b.Bool
	case StringKind:
		return a.Str == b.Str
	}
	return false
}

// isOrdered reports whether values of this kind support ordering comparisons.
func (k ValueKind) isOrdered() bool {
	return k == NumberKind || k == SizeKind || k == DurationKind
}

// normalizeDynamicName normalizes a dynamic block/reference value into a
// valid resource name: it trims surrounding whitespace, lowercases, replaces
// each run of invalid characters (anything other than a-z, 0-9 or '-') with a
// single '-', trims leading and trailing '-', and rejects an empty result.
func normalizeDynamicName(s string) (string, error) {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevDash = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "", fmt.Errorf("value %q normalizes to an empty resource name", s)
	}
	return name, nil
}

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
