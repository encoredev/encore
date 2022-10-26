package errinsrc

import (
	"fmt"
	"path"
	"strings"
	"testing"

	. "encr.dev/pkg/errinsrc/internal"
	"encr.dev/pkg/golden"
)

func Test_renderSrc_Simple(t *testing.T) {
	testParams := []struct {
		testName     string
		line, column int
		level        LocationType
		message      string
	}{
		{testName: "error no text", line: 5, column: 7, level: LocError, message: ""},
		{testName: "simple error", line: 5, column: 2, level: LocError, message: "What is a foo?"},
		{testName: "simple warning", line: 10, column: 7, level: LocWarning, message: "You sure about this?"},
		{testName: "simple help", line: 13, column: 4, level: LocHelp, message: "help: try a switch statement here"},
		{testName: "single character error", line: 6, column: 11, level: LocError, message: "This looks dodgy"},
		{testName: "multiline message", line: 1, column: 9, level: LocError, message: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Aenean interdum porttitor elementum. Duis eget cursus arcu, ut interdum lorem. Suspendisse vel diam et eros cursus vestibulum at et lacus. Nullam felis tellus, cursus nec arcu nec, laoreet maximus ante. Cras pellentesque est est, nec laoreet magna accumsan vel. Mauris commodo dui purus, non ullamcorper nisl commodo auctor. Vivamus finibus mi ut risus tempor pellentesque. Nulla facilisi. Nullam rhoncus neque porta erat molestie, at malesuada est aliquam. Etiam convallis lorem eget euismod eleifend. Phasellus sit amet diam in orci molestie pulvinar. Vestibulum non auctor dolor, vel imperdiet risus. Nam egestas et purus id sodales. Aliquam in metus varius, porta mi nec, ornare mauris.\n\nQuisque eu nisi vel nulla sodales pretium sed a dolor. Morbi convallis ornare ligula, ut aliquam velit auctor id. In in neque turpis. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Cras et arcu id magna accumsan semper. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Nunc semper rhoncus tincidunt. Vestibulum lacus erat, molestie ut ultrices convallis, placerat at lacus. Integer tempus tempus sodales. Integer sit amet est quam. Praesent vitae condimentum mi.\n\n"},
	}

	for _, tp := range testParams {
		tp := tp
		t.Run(tp.testName, func(t *testing.T) {
			loc := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), tp.line, tp.column}, testDataFullPath)
			loc.Text = tp.message
			loc.Type = tp.level

			err := New(ErrParams{
				Code:      1,
				Title:     "simple test error",
				Summary:   "There has been a simple error in your code",
				Detail:    "For more information please visit our great help documentation",
				Locations: SrcLocations{loc},
			}, false)

			testError(t, err)
		})
	}
}

func Test_renderSrc_MultipleSeperateInSameFile(t *testing.T) {
	t.Run("spaced apart", func(t *testing.T) {
		loc1 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 5, 7}, testDataFullPath)
		loc1.Text = "defined as a boolean here"
		loc2 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 13, 9}, testDataFullPath)
		loc2.Text = "referenced here"

		err := New(ErrParams{
			Code:      1,
			Title:     "simple test error",
			Summary:   "There has been a simple error in your code",
			Detail:    "For more information please visit our great help documentation",
			Locations: SrcLocations{loc1, loc2},
		}, false)

		testError(t, err)
	})
	t.Run("on following lines", func(t *testing.T) {
		loc1 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 5, 7}, testDataFullPath)
		loc1.Text = "this is weird"
		loc2 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 6, 13}, testDataFullPath)
		loc2.Text = "so is this!"

		err := New(ErrParams{
			Code:      1,
			Title:     "simple test error",
			Summary:   "There has been a simple error in your code",
			Detail:    "For more information please visit our great help documentation",
			Locations: SrcLocations{loc1, loc2},
		}, false)

		testError(t, err)
	})
	t.Run("on same line", func(t *testing.T) {
		loc1 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 5, 7}, testDataFullPath)
		loc1.Text = "hint: change this to an int"
		loc1.Type = LocHelp
		loc2 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 5, 2}, testDataFullPath)
		loc2.Text = "wrong type here"

		err := New(ErrParams{
			Code:      1,
			Title:     "simple test error",
			Summary:   "There has been a simple error in your code",
			Detail:    "For more information please visit our great help documentation",
			Locations: SrcLocations{loc1, loc2},
		}, false)

		testError(t, err)
	})
}

func Test_renderSrc_MutlilineError(t *testing.T) {
	loc1 := FromCueTokenPos(&errorLoc{path.Join(testDataFullPath, "test.cue"), 4, 7}, testDataFullPath)
	loc1.End = Pos{7, 1}
	loc1.Text = "this is an error which spans multiple lines\nlike this so we can test alignment"

	err := New(ErrParams{
		Code:      1,
		Title:     "simple test error",
		Summary:   "There has been a simple error in your code",
		Detail:    "For more information please visit our great help documentation",
		Locations: SrcLocations{loc1},
	}, false)

	testError(t, err)
}

/* helpers */

func testError(t *testing.T, err *ErrInSrc) {
	// Reset for stdout with colours
	set = unicodeSet
	ColoursInErrors(true)
	fmt.Println(err.Error())

	// Now golden files without colours
	goldenFile := strings.Replace(t.Name(), "/", "__", -1)
	ColoursInErrors(false)
	golden.TestAgainst(t, goldenFile+"_unicode.golden", err.Error())
	set = asciiSet
	golden.TestAgainst(t, goldenFile+"_ascii.golden", err.Error())
}

type errorLoc struct {
	filename string
	line     int
	column   int
}

func (e *errorLoc) Filename() string {
	return e.filename
}

func (e *errorLoc) Line() int {
	return e.line
}

func (e *errorLoc) Column() int {
	return e.column
}
