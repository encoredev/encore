package ecl

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParseQuantity(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	v := MustParseQuantity("512Mi")
	c.Assert(v.Kind, qt.Equals, SizeKind)
	c.Assert(v.Num, qt.Equals, float64(512*1024*1024))
	c.Assert(v.String(), qt.Equals, "512Mi")

	v = MustParseQuantity("30d")
	c.Assert(v.Kind, qt.Equals, DurationKind)
	c.Assert(v.Num, qt.Equals, float64(30*24*60*60*1000))
	c.Assert(v.String(), qt.Equals, "30d")

	v = MustParseQuantity("2.5")
	c.Assert(v.Kind, qt.Equals, NumberKind)
	c.Assert(v.Num, qt.Equals, 2.5)
	c.Assert(v.String(), qt.Equals, "2.5")

	_, err := ParseQuantity("10grams")
	c.Assert(err, qt.ErrorMatches, `unknown unit "grams".*`)
	_, err = ParseQuantity("abc")
	c.Assert(err, qt.ErrorMatches, `invalid quantity "abc"`)
}

func TestValueEquality(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	// Different unit spellings of the same quantity are equal.
	c.Assert(valuesEqual(MustParseQuantity("1Gi"), MustParseQuantity("1024Mi")), qt.IsTrue)
	c.Assert(valuesEqual(MustParseQuantity("1m"), MustParseQuantity("60s")), qt.IsTrue)
	c.Assert(valuesEqual(MustParseQuantity("1GB"), MustParseQuantity("1Gi")), qt.IsFalse)

	// Different kinds are never equal, even with the same numeric value.
	c.Assert(valuesEqual(Number(1024), MustParseQuantity("1024B")), qt.IsFalse)
	c.Assert(valuesEqual(Bool(true), String("true")), qt.IsFalse)
}

func TestValueString(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	c.Assert(Number(0.5).String(), qt.Equals, "0.5")
	c.Assert(Number(8).String(), qt.Equals, "8")
	c.Assert(Bool(false).String(), qt.Equals, "false")
	c.Assert(String("europe-west1").String(), qt.Equals, `"europe-west1"`)

	// Values without a display unit pick a sensible one automatically.
	c.Assert(Value{Kind: SizeKind, Num: 2048}.String(), qt.Equals, "2Ki")
	c.Assert(Value{Kind: DurationKind, Num: 90_000}.String(), qt.Equals, "90s")
}

func TestSuggest(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	kinds := sortedKinds(DefaultSchema)
	c.Assert(suggest("sevice", kinds), qt.Equals, "service")
	c.Assert(suggest("buckte", kinds), qt.Equals, "bucket")
	c.Assert(suggest("zzzzz", kinds), qt.Equals, "")
	// Prefix matches are preferred among equally distant candidates.
	c.Assert(suggest("G", []string{"B", "GB", "Gi", "KB"}), qt.Equals, "GB")
}
